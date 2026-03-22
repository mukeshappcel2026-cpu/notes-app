const AWS = require('aws-sdk');
const config = require('../config');

let docClient;

function getDocClient() {
  if (!docClient) {
    const opts = { region: config.aws.region };
    if (config.aws.dynamoEndpoint) opts.endpoint = config.aws.dynamoEndpoint;
    docClient = new AWS.DynamoDB.DocumentClient(opts);
  }
  return docClient;
}

// Deduplication works by maintaining "detection groups."
// A group is keyed by (tenantId, sourceIp, providerId).
// Instead of creating a new finding per event, we:
//   1. Check if an active group exists for this (source, provider) pair
//   2. If yes: increment the count, update lastSeenAt
//   3. If no: create a new group + the first finding
//
// Groups have a configurable window (default 1 hour).
// After the window closes, a new group starts.

const DEFAULT_GROUP_WINDOW_MS = 60 * 60 * 1000; // 1 hour

async function findActiveGroup(tenantId, sourceIp, providerId) {
  const db = getDocClient();
  const groupKey = buildGroupKey(sourceIp, providerId);

  const result = await db.query({
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': `GROUP#${groupKey}#`,
    },
    ScanIndexForward: false, // newest first
    Limit: 1,
  }).promise();

  if (!result.Items || result.Items.length === 0) return null;

  const group = result.Items[0];
  const windowEnd = new Date(group.windowStart).getTime() + (group.windowMs || DEFAULT_GROUP_WINDOW_MS);

  if (Date.now() > windowEnd) {
    // Window has expired, no active group
    return null;
  }

  return group;
}

async function createGroup(tenantId, finding) {
  const db = getDocClient();
  const groupKey = buildGroupKey(finding.sourceIp, finding.providerId);
  const now = new Date().toISOString();

  const group = {
    PK: `TENANT#${tenantId}`,
    SK: `GROUP#${groupKey}#${now}`,
    groupKey,
    tenantId,
    status: 'active',

    // Group identity
    sourceIp: finding.sourceIp,
    providerId: finding.providerId,
    providerName: finding.providerName,
    riskTier: finding.riskTier,
    category: finding.category,

    // Enrichment (set during creation, updated if better data arrives)
    serviceName: finding.serviceName || null,
    team: finding.team || null,
    environment: finding.environment || null,
    instanceId: finding.instanceId || null,

    // Aggregation
    eventCount: 1,
    detectionMethods: [finding.detectionMethod],
    domains: finding.queryName ? [finding.queryName] : [],

    // Window
    windowStart: now,
    windowMs: DEFAULT_GROUP_WINDOW_MS,
    firstSeenAt: now,
    lastSeenAt: now,

    // First finding reference
    firstFindingId: finding.findingId,

    // Timestamps
    createdAt: now,
    updatedAt: now,
    acknowledgedAt: null,
    acknowledgedBy: null,
  };

  await db.put({ TableName: config.tables.findings, Item: group }).promise();
  return group;
}

async function incrementGroup(tenantId, group, event) {
  const db = getDocClient();
  const now = new Date().toISOString();

  const updateExpr = [
    'SET eventCount = eventCount + :one',
    'lastSeenAt = :now',
    'updatedAt = :now',
  ];
  const exprValues = {
    ':one': 1,
    ':now': now,
  };

  // Add new detection method if not already tracked
  if (event.detectionMethod && !group.detectionMethods.includes(event.detectionMethod)) {
    updateExpr.push('detectionMethods = list_append(detectionMethods, :newMethod)');
    exprValues[':newMethod'] = [event.detectionMethod];
  }

  // Add new domain if not already tracked
  const domain = event.queryName || event.sniValue;
  if (domain && !group.domains.includes(domain)) {
    updateExpr.push('domains = list_append(domains, :newDomain)');
    exprValues[':newDomain'] = [domain];
  }

  // Update enrichment if we got better data
  if (event.serviceName && !group.serviceName) {
    updateExpr.push('serviceName = :serviceName');
    exprValues[':serviceName'] = event.serviceName;
  }
  if (event.team && !group.team) {
    updateExpr.push('team = :team');
    exprValues[':team'] = event.team;
  }

  const result = await db.update({
    TableName: config.tables.findings,
    Key: { PK: group.PK, SK: group.SK },
    UpdateExpression: updateExpr.join(', '),
    ExpressionAttributeValues: exprValues,
    ReturnValues: 'ALL_NEW',
  }).promise();

  return result.Attributes;
}

async function getGroups(tenantId, { status, providerId, limit = 50, lastKey } = {}) {
  const db = getDocClient();
  const params = {
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'GROUP#',
    },
    ScanIndexForward: false,
    Limit: limit,
  };

  const filters = [];
  if (status) {
    filters.push('#status = :status');
    params.ExpressionAttributeValues[':status'] = status;
    params.ExpressionAttributeNames = { '#status': 'status' };
  }
  if (providerId) {
    filters.push('providerId = :providerId');
    params.ExpressionAttributeValues[':providerId'] = providerId;
  }
  if (filters.length > 0) {
    params.FilterExpression = filters.join(' AND ');
  }
  if (lastKey) {
    params.ExclusiveStartKey = JSON.parse(Buffer.from(lastKey, 'base64').toString());
  }

  const result = await db.query(params).promise();

  return {
    groups: result.Items || [],
    nextKey: result.LastEvaluatedKey
      ? Buffer.from(JSON.stringify(result.LastEvaluatedKey)).toString('base64')
      : null,
  };
}

async function acknowledgeGroup(tenantId, groupKey, acknowledgedBy) {
  const db = getDocClient();

  // Find the group by its groupKey
  const query = await db.query({
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': `GROUP#${groupKey}#`,
    },
    ScanIndexForward: false,
    Limit: 1,
  }).promise();

  if (!query.Items || query.Items.length === 0) return null;

  const group = query.Items[0];
  const result = await db.update({
    TableName: config.tables.findings,
    Key: { PK: group.PK, SK: group.SK },
    UpdateExpression: 'SET #status = :status, acknowledgedAt = :at, acknowledgedBy = :by',
    ExpressionAttributeNames: { '#status': 'status' },
    ExpressionAttributeValues: {
      ':status': 'acknowledged',
      ':at': new Date().toISOString(),
      ':by': acknowledgedBy,
    },
    ReturnValues: 'ALL_NEW',
  }).promise();

  return result.Attributes;
}

async function getGroupStats(tenantId) {
  const db = getDocClient();

  const result = await db.query({
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'GROUP#',
    },
  }).promise();

  const groups = result.Items || [];
  const stats = {
    totalGroups: groups.length,
    activeGroups: 0,
    acknowledgedGroups: 0,
    totalEvents: 0,
    byProvider: {},
    byTeam: {},
    byService: {},
    byRiskTier: { high: 0, medium: 0, low: 0 },
    topSources: [],
  };

  const sourceMap = new Map();

  for (const group of groups) {
    if (group.status === 'active') stats.activeGroups++;
    if (group.status === 'acknowledged') stats.acknowledgedGroups++;
    stats.totalEvents += group.eventCount || 1;

    stats.byProvider[group.providerName] = (stats.byProvider[group.providerName] || 0) + (group.eventCount || 1);
    if (group.riskTier) stats.byRiskTier[group.riskTier] += group.eventCount || 1;
    if (group.team) stats.byTeam[group.team] = (stats.byTeam[group.team] || 0) + (group.eventCount || 1);
    if (group.serviceName) stats.byService[group.serviceName] = (stats.byService[group.serviceName] || 0) + (group.eventCount || 1);

    const existing = sourceMap.get(group.sourceIp) || { ip: group.sourceIp, service: group.serviceName, team: group.team, events: 0, providers: new Set() };
    existing.events += group.eventCount || 1;
    existing.providers.add(group.providerName);
    sourceMap.set(group.sourceIp, existing);
  }

  stats.topSources = Array.from(sourceMap.values())
    .map((s) => ({ ...s, providers: Array.from(s.providers) }))
    .sort((a, b) => b.events - a.events)
    .slice(0, 10);

  return stats;
}

function buildGroupKey(sourceIp, providerId) {
  return `${sourceIp}::${providerId}`;
}

module.exports = {
  findActiveGroup,
  createGroup,
  incrementGroup,
  getGroups,
  acknowledgeGroup,
  getGroupStats,
  buildGroupKey,
  DEFAULT_GROUP_WINDOW_MS,
};
