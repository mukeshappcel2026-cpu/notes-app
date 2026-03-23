/**
 * Shadow AI Detection — End-to-End Simulation
 *
 * This test simulates a realistic enterprise scenario:
 *   - 6 different apps making unauthorized AI API calls
 *   - 1 legitimate gateway that should be allowlisted
 *   - Mix of DNS Firewall alerts, Network Firewall SNI alerts, and EventBridge events
 *   - Validates detection, deduplication, enrichment, and stats
 *
 * Run: NODE_ENV=test npx jest tests/simulation/simulate.test.js --verbose
 */

const request = require('supertest');
const AWSMock = require('aws-sdk-mock');
const AWS = require('aws-sdk');
const app = require('../../src/app');
const { SHADOW_APPS, wrapInEventBridge } = require('./traffic-profiles');

// ─────────────────────────────────────────────────
// Mock DynamoDB — in-memory store
// ─────────────────────────────────────────────────
const tables = {};

function getTable(tableName) {
  if (!tables[tableName]) tables[tableName] = [];
  return tables[tableName];
}

function matchesKey(item, key) {
  return Object.keys(key).every((k) => item[k] === key[k]);
}

beforeAll(() => {
  AWSMock.setSDKInstance(AWS);

  // PUT — store item
  AWSMock.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
    const table = getTable(params.TableName);

    // Handle ConditionExpression for "attribute_not_exists(PK)" (seed idempotency)
    if (params.ConditionExpression === 'attribute_not_exists(PK)') {
      const exists = table.find((i) => i.PK === params.Item.PK && i.SK === params.Item.SK);
      if (exists) {
        return callback({ code: 'ConditionalCheckFailedException' });
      }
    }

    // Upsert
    const idx = table.findIndex((i) => i.PK === params.Item.PK && i.SK === params.Item.SK);
    if (idx >= 0) {
      table[idx] = params.Item;
    } else {
      table.push(params.Item);
    }
    callback(null, {});
  });

  // GET — retrieve single item
  AWSMock.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
    const table = getTable(params.TableName);
    const item = table.find((i) => matchesKey(i, params.Key));
    callback(null, { Item: item || null });
  });

  // QUERY — query by PK and SK prefix
  AWSMock.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
    const table = getTable(params.TableName);
    const pkValue = params.ExpressionAttributeValues[':pk'];
    const skPrefix = params.ExpressionAttributeValues[':prefix'] || '';

    let items = table.filter((i) => {
      let match = i.PK === pkValue;
      if (skPrefix) match = match && i.SK.startsWith(skPrefix);
      return match;
    });

    // Apply FilterExpression (basic support)
    if (params.FilterExpression && params.ExpressionAttributeValues) {
      items = items.filter((item) => {
        let pass = true;
        if (params.ExpressionAttributeValues[':status']) {
          pass = pass && item.status === params.ExpressionAttributeValues[':status'];
        }
        if (params.ExpressionAttributeValues[':providerId']) {
          pass = pass && item.providerId === params.ExpressionAttributeValues[':providerId'];
        }
        if (params.ExpressionAttributeValues[':findingId']) {
          pass = pass && item.findingId === params.ExpressionAttributeValues[':findingId'];
        }
        if (params.ExpressionAttributeValues[':team']) {
          pass = pass && item.team === params.ExpressionAttributeValues[':team'];
        }
        return pass;
      });
    }

    // Sort
    if (params.ScanIndexForward === false) {
      items.sort((a, b) => b.SK.localeCompare(a.SK));
    } else {
      items.sort((a, b) => a.SK.localeCompare(b.SK));
    }

    // Limit
    if (params.Limit) {
      items = items.slice(0, params.Limit);
    }

    callback(null, { Items: items, Count: items.length });
  });

  // UPDATE — update item
  AWSMock.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
    const table = getTable(params.TableName);
    const item = table.find((i) => matchesKey(i, params.Key));
    if (!item) return callback(null, { Attributes: null });

    // Parse basic SET expressions
    const values = params.ExpressionAttributeValues || {};
    const names = params.ExpressionAttributeNames || {};

    // Handle eventCount = eventCount + :one
    if (values[':one']) {
      item.eventCount = (item.eventCount || 0) + values[':one'];
    }
    if (values[':now']) {
      item.lastSeenAt = values[':now'];
      item.updatedAt = values[':now'];
    }
    if (values[':status']) {
      item.status = values[':status'];
    }
    if (values[':at']) {
      item.acknowledgedAt = values[':at'];
    }
    if (values[':by']) {
      item.acknowledgedBy = values[':by'];
    }
    if (values[':newMethod'] && Array.isArray(item.detectionMethods)) {
      item.detectionMethods.push(...values[':newMethod']);
    }
    if (values[':newDomain'] && Array.isArray(item.domains)) {
      item.domains.push(...values[':newDomain']);
    }
    if (values[':serviceName'] && !item.serviceName) {
      item.serviceName = values[':serviceName'];
    }
    if (values[':team'] && !item.team) {
      item.team = values[':team'];
    }

    callback(null, { Attributes: { ...item } });
  });

  // DELETE
  AWSMock.mock('DynamoDB.DocumentClient', 'delete', (params, callback) => {
    const table = getTable(params.TableName);
    const idx = table.findIndex((i) => matchesKey(i, params.Key));
    if (idx >= 0) table.splice(idx, 1);
    callback(null, {});
  });

  // Mock EC2 (enrichment will try to call this)
  AWSMock.mock('EC2', 'describeInstances', (params, callback) => {
    const ipFilter = params.Filters.find((f) => f.Name === 'private-ip-address');
    const ip = ipFilter ? ipFilter.Values[0] : null;

    // Simulate EC2 responses for known IPs
    const instances = {
      '10.0.5.42': {
        InstanceId: 'i-payments-001',
        OwnerId: '123456789012',
        Tags: [
          { Key: 'Name', Value: 'payments-api-prod' },
          { Key: 'service', Value: 'payments-api' },
          { Key: 'team', Value: 'payments' },
          { Key: 'environment', Value: 'production' },
        ],
      },
      '10.0.6.10': {
        InstanceId: 'i-search-001',
        OwnerId: '123456789012',
        Tags: [
          { Key: 'Name', Value: 'search-indexer-prod' },
          { Key: 'service', Value: 'search-indexer' },
          { Key: 'team', Value: 'search' },
          { Key: 'environment', Value: 'production' },
        ],
      },
      '10.0.8.55': {
        InstanceId: 'i-marketing-001',
        OwnerId: '123456789012',
        Tags: [
          { Key: 'Name', Value: 'marketing-bot-prod' },
          { Key: 'service', Value: 'marketing-bot' },
          { Key: 'team', Value: 'marketing' },
          { Key: 'environment', Value: 'production' },
        ],
      },
      '10.0.7.22': {
        InstanceId: 'i-internal-001',
        OwnerId: '123456789012',
        Tags: [
          { Key: 'Name', Value: 'internal-tool-staging' },
          { Key: 'service', Value: 'admin-dashboard' },
          { Key: 'team', Value: 'platform-eng' },
          { Key: 'environment', Value: 'staging' },
        ],
      },
      '10.0.9.99': {
        InstanceId: 'i-dev-099',
        OwnerId: '123456789012',
        Tags: [
          { Key: 'Name', Value: 'dev-box-jsmith' },
          { Key: 'team', Value: 'engineering' },
          { Key: 'environment', Value: 'development' },
        ],
      },
    };

    const instance = instances[ip];
    if (instance) {
      callback(null, { Reservations: [{ Instances: [instance] }] });
    } else {
      callback(null, { Reservations: [] });
    }
  });
});

afterAll(() => {
  AWSMock.restore();
});

beforeEach(() => {
  // Clear all tables between tests
  Object.keys(tables).forEach((k) => { tables[k] = []; });
});

const TENANT = 'acme-corp';
const authHeader = ['X-Tenant-Id', TENANT];

// ─────────────────────────────────────────────────
// SIMULATION
// ─────────────────────────────────────────────────

describe('Shadow AI Detection — Full Simulation', () => {

  // Seed the provider registry first
  beforeEach(async () => {
    const { seedRegistry } = require('../../src/services/registry.service');
    await seedRegistry();
  });

  // ─── SCENARIO 1: Basic Detection ───
  describe('Scenario 1: Detect payments-api calling OpenAI', () => {
    it('detects DNS Firewall alert and creates finding with enrichment', async () => {
      const event = SHADOW_APPS[0].events[0]; // payments-api → api.openai.com

      const res = await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send(event);

      expect(res.status).toBe(201);
      expect(res.body.action).toBe('finding_created');
      expect(res.body.provider).toBe('OpenAI');
      expect(res.body.riskTier).toBe('high');
      expect(res.body.sourceIp).toBe('10.0.5.42');
      expect(res.body.serviceName).toBe('payments-api');
      expect(res.body.team).toBe('payments');
    });
  });

  // ─── SCENARIO 2: Deduplication ───
  describe('Scenario 2: Dedup — 5 events from payments-api become 1 finding', () => {
    it('creates 1 finding + 4 grouped events', async () => {
      const events = SHADOW_APPS[0].events; // 5 events from payments-api

      const results = [];
      for (const event of events) {
        const res = await request(app)
          .post('/api/v1/ingest/event')
          .set(...authHeader)
          .send(event);
        results.push(res.body);
      }

      // First event creates a finding
      expect(results[0].action).toBe('finding_created');
      expect(results[0].findingId).toBeDefined();

      // Remaining 4 events are grouped (deduplicated)
      for (let i = 1; i < results.length; i++) {
        expect(results[i].action).toBe('grouped');
        expect(results[i].eventCount).toBe(i + 1);
      }

      // Last group increment should show count=5
      expect(results[4].eventCount).toBe(5);
    });
  });

  // ─── SCENARIO 3: Allowlist (Gateway) ───
  describe('Scenario 3: Gateway traffic is allowlisted', () => {
    it('skips findings for allowlisted gateway IP', async () => {
      // First, add the gateway IP to the allowlist
      const allowRes = await request(app)
        .post('/api/v1/allowlist')
        .set(...authHeader)
        .send({
          type: 'source_ip',
          value: '10.0.3.10',
          reason: 'AI Gateway — production LiteLLM proxy',
        });
      expect(allowRes.status).toBe(201);

      // Now send events from the gateway IP
      const gatewayEvents = SHADOW_APPS[3].events; // ml-pipeline (gateway)
      for (const event of gatewayEvents) {
        const res = await request(app)
          .post('/api/v1/ingest/event')
          .set(...authHeader)
          .send(event);

        expect(res.body.action).toBe('allowlisted');
        expect(res.body.reason).toBe('AI Gateway — production LiteLLM proxy');
      }
    });
  });

  // ─── SCENARIO 4: Multiple Providers from Same Source ───
  describe('Scenario 4: Rogue developer calling 3 providers', () => {
    it('creates separate findings per provider from same IP', async () => {
      const events = SHADOW_APPS[5].events; // rogue-developer

      const results = [];
      for (const event of events) {
        const res = await request(app)
          .post('/api/v1/ingest/event')
          .set(...authHeader)
          .send(event);
        results.push(res.body);
      }

      // First 3 events: different providers → 3 separate findings
      expect(results[0].action).toBe('finding_created');
      expect(results[0].provider).toBe('OpenAI');

      expect(results[1].action).toBe('finding_created');
      expect(results[1].provider).toBe('Anthropic');

      expect(results[2].action).toBe('finding_created');
      expect(results[2].provider).toBe('Mistral AI');

      // 4th event: OpenAI again via SNI → should be grouped with first
      expect(results[3].action).toBe('grouped');
      expect(results[3].groupKey).toBe('10.0.9.99::openai');
      expect(results[3].eventCount).toBe(2);
    });
  });

  // ─── SCENARIO 5: Wildcard Domain Match ───
  describe('Scenario 5: Azure OpenAI wildcard subdomain detection', () => {
    it('detects customer-specific Azure OpenAI deployment', async () => {
      const event = SHADOW_APPS[4].events[0]; // acme-gpt4.openai.azure.com

      const res = await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send(event);

      expect(res.status).toBe(201);
      expect(res.body.action).toBe('finding_created');
      expect(res.body.provider).toBe('Azure OpenAI');
      expect(res.body.serviceName).toBe('admin-dashboard');
      expect(res.body.team).toBe('platform-eng');
    });
  });

  // ─── SCENARIO 6: Network Firewall SNI Detection ───
  describe('Scenario 6: Detect via Network Firewall SNI (not DNS)', () => {
    it('detects DeepSeek via SNI inspection', async () => {
      const event = SHADOW_APPS[2].events[0]; // marketing-bot → deepseek SNI

      const res = await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send(event);

      expect(res.status).toBe(201);
      expect(res.body.action).toBe('finding_created');
      expect(res.body.provider).toBe('DeepSeek');
      expect(res.body.riskTier).toBe('high');
      expect(res.body.serviceName).toBe('marketing-bot');
      expect(res.body.team).toBe('marketing');
    });
  });

  // ─── SCENARIO 7: Batch Ingestion ───
  describe('Scenario 7: Batch ingestion from EventBridge', () => {
    it('processes batch of mixed events correctly', async () => {
      // Mix events from different apps
      const events = [
        SHADOW_APPS[0].events[0], // payments → openai
        SHADOW_APPS[1].events[0], // search → anthropic
        SHADOW_APPS[2].events[0], // marketing → deepseek
        { queryName: 'google.com', sourceIp: '10.0.1.1', firewallRuleAction: 'ALERT' }, // non-AI
      ];

      const res = await request(app)
        .post('/api/v1/ingest/batch')
        .set(...authHeader)
        .send({ events });

      expect(res.status).toBe(200);
      expect(res.body.summary.total).toBe(4);
      expect(res.body.summary.findings_created).toBe(3); // 3 AI providers
      expect(res.body.summary.skipped).toBe(1); // google.com not in registry
    });
  });

  // ─── SCENARIO 8: EventBridge Format ───
  describe('Scenario 8: EventBridge envelope format', () => {
    it('unwraps EventBridge envelope and detects correctly', async () => {
      const event = SHADOW_APPS[0].events[0];
      const envelope = {
        version: '0',
        id: 'evt-test-123',
        source: 'aws.route53resolver',
        'detail-type': 'DNS Firewall Alert',
        account: '123456789012',
        region: 'us-east-1',
        time: event.timestamp,
        detail: {
          'query-name': event.queryName,
          'source-ip': event.sourceIp,
          'vpc-id': event.vpcId,
          'firewall-rule-action': 'ALERT',
        },
      };

      const res = await request(app)
        .post('/api/v1/ingest/eventbridge')
        .set(...authHeader)
        .send(envelope);

      expect(res.status).toBe(201);
      expect(res.body.action).toBe('finding_created');
      expect(res.body.provider).toBe('OpenAI');
    });
  });

  // ─── SCENARIO 9: Full Pipeline — Stats After Simulation ───
  describe('Scenario 9: Full pipeline — send all traffic, verify stats', () => {
    it('runs full simulation and produces accurate stats', async () => {
      // Step 1: Allowlist the gateway
      await request(app)
        .post('/api/v1/allowlist')
        .set(...authHeader)
        .send({ type: 'source_ip', value: '10.0.3.10', reason: 'AI Gateway' });

      // Step 2: Send ALL events from ALL apps
      for (const appProfile of SHADOW_APPS) {
        for (const event of appProfile.events) {
          await request(app)
            .post('/api/v1/ingest/event')
            .set(...authHeader)
            .send(event);
        }
      }

      // Step 3: Check group stats
      const statsRes = await request(app)
        .get('/api/v1/groups/stats')
        .set(...authHeader);

      expect(statsRes.status).toBe(200);
      const stats = statsRes.body;

      // We expect groups from 5 apps (gateway is allowlisted)
      // payments-api: 1 group (5 events → openai)
      // search-indexer: 1 group (2 events → anthropic)
      // marketing-bot: 2 groups (deepseek api + deepseek chat = different domains but same provider, so 1 group)
      // internal-tool: 1 group (1 event → azure-openai)
      // rogue-developer: 3 groups (openai, anthropic, mistral from same IP)
      expect(stats.totalGroups).toBeGreaterThanOrEqual(6);
      expect(stats.totalEvents).toBeGreaterThanOrEqual(14); // 16 total - 2 allowlisted

      // Verify provider breakdown exists
      expect(stats.byProvider).toBeDefined();
      expect(stats.byProvider['OpenAI']).toBeGreaterThanOrEqual(1);
      expect(stats.byProvider['Anthropic']).toBeGreaterThanOrEqual(1);
      expect(stats.byProvider['DeepSeek']).toBeGreaterThanOrEqual(1);

      // Verify top sources
      expect(stats.topSources).toBeDefined();
      expect(stats.topSources.length).toBeGreaterThanOrEqual(1);

      // Step 4: Check findings can be filtered by team
      const findingsRes = await request(app)
        .get('/api/v1/findings?team=payments')
        .set(...authHeader);

      expect(findingsRes.status).toBe(200);
      expect(findingsRes.body.findings.length).toBeGreaterThanOrEqual(1);
      expect(findingsRes.body.findings[0].team).toBe('payments');
      expect(findingsRes.body.findings[0].serviceName).toBe('payments-api');

      // Step 5: Check groups list
      const groupsRes = await request(app)
        .get('/api/v1/groups')
        .set(...authHeader);

      expect(groupsRes.status).toBe(200);
      expect(groupsRes.body.groups.length).toBeGreaterThanOrEqual(6);

      // Verify payments group has been deduped
      const paymentsGroup = groupsRes.body.groups.find(
        (g) => g.sourceIp === '10.0.5.42' && g.providerId === 'openai'
      );
      expect(paymentsGroup).toBeDefined();
      expect(paymentsGroup.eventCount).toBe(5);
      expect(paymentsGroup.serviceName).toBe('payments-api');
      expect(paymentsGroup.team).toBe('payments');

      // Step 6: Verify non-AI traffic was ignored
      // Send a non-AI domain
      const nonAiRes = await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send({
          queryName: 'stackoverflow.com',
          sourceIp: '10.0.1.1',
          firewallRuleAction: 'ALERT',
        });
      expect(nonAiRes.body.action).toBe('skipped');
      expect(nonAiRes.body.reason).toBe('no_provider_match');
    });
  });

  // ─── SCENARIO 10: Service Catalog Enrichment ───
  describe('Scenario 10: Manual service catalog enrichment', () => {
    it('enriches findings from service catalog when EC2 lookup misses', async () => {
      // Register a service mapping for an IP that EC2 won't know about (e.g., ECS Fargate)
      const catalogRes = await request(app)
        .post('/api/v1/enrichment/catalog')
        .set(...authHeader)
        .send({
          ipOrCidr: '10.0.12.88',
          serviceName: 'fargate-summarizer',
          team: 'data-science',
          environment: 'production',
          namespace: 'ml-workloads',
        });
      expect(catalogRes.status).toBe(201);

      // Now send an event from that IP
      const res = await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send({
          queryName: 'api.cohere.ai',
          sourceIp: '10.0.12.88',
          vpcId: 'vpc-prod-001',
          firewallRuleAction: 'ALERT',
        });

      expect(res.status).toBe(201);
      expect(res.body.action).toBe('finding_created');
      expect(res.body.provider).toBe('Cohere');
      // Service catalog should have enriched this
      expect(res.body.serviceName).toBe('fargate-summarizer');
      expect(res.body.team).toBe('data-science');
    });
  });

  // ─── SCENARIO 11: Acknowledge a group ───
  describe('Scenario 11: Acknowledge detection group', () => {
    it('acknowledges a group and it no longer shows as active', async () => {
      // Create a detection
      await request(app)
        .post('/api/v1/ingest/event')
        .set(...authHeader)
        .send(SHADOW_APPS[1].events[0]); // search-indexer → anthropic

      // Get groups
      const groupsRes = await request(app)
        .get('/api/v1/groups')
        .set(...authHeader);

      const group = groupsRes.body.groups[0];
      expect(group.status).toBe('active');

      // Acknowledge it
      const ackRes = await request(app)
        .post(`/api/v1/groups/${encodeURIComponent(group.groupKey)}/acknowledge`)
        .set(...authHeader)
        .send({ acknowledgedBy: 'security-team@acme.com' });

      expect(ackRes.status).toBe(200);
      expect(ackRes.body.status).toBe('acknowledged');
      expect(ackRes.body.acknowledgedBy).toBe('security-team@acme.com');
    });
  });
});
