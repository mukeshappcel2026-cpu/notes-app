const AWS = require('aws-sdk');
const config = require('../config');

let ec2Client;

function getEC2Client() {
  if (!ec2Client) {
    ec2Client = new AWS.EC2({ region: config.aws.region });
  }
  return ec2Client;
}

// In-memory cache: IP → enrichment data
// TTL-based, refreshed when stale
const enrichmentCache = new Map();
const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

async function enrichByIp(tenantId, sourceIp, vpcId) {
  const cacheKey = `${tenantId}:${sourceIp}`;
  const cached = enrichmentCache.get(cacheKey);
  if (cached && Date.now() - cached.timestamp < CACHE_TTL_MS) {
    return cached.data;
  }

  const enrichment = {
    sourceIp,
    instanceId: null,
    instanceName: null,
    serviceName: null,
    team: null,
    environment: null,
    podName: null,
    namespace: null,
    accountId: null,
    enrichedAt: new Date().toISOString(),
    enrichmentSource: 'none',
  };

  // Try EC2 enrichment first
  try {
    const ec2Data = await lookupEC2Instance(sourceIp, vpcId);
    if (ec2Data) {
      Object.assign(enrichment, ec2Data);
      enrichment.enrichmentSource = 'ec2';
    }
  } catch (err) {
    // EC2 enrichment is best-effort; log and continue
    console.error(`EC2 enrichment failed for ${sourceIp}:`, err.message);
  }

  // Try tenant-provided service catalog as fallback
  try {
    const catalogData = await lookupServiceCatalog(tenantId, sourceIp);
    if (catalogData) {
      // Catalog data fills gaps but doesn't overwrite EC2 data
      for (const [key, value] of Object.entries(catalogData)) {
        if (value && !enrichment[key]) {
          enrichment[key] = value;
        }
      }
      if (enrichment.enrichmentSource === 'none') {
        enrichment.enrichmentSource = 'service_catalog';
      }
    }
  } catch (err) {
    console.error(`Service catalog lookup failed for ${sourceIp}:`, err.message);
  }

  enrichmentCache.set(cacheKey, { data: enrichment, timestamp: Date.now() });
  return enrichment;
}

async function lookupEC2Instance(sourceIp, vpcId) {
  const ec2 = getEC2Client();

  const filters = [
    { Name: 'private-ip-address', Values: [sourceIp] },
  ];
  if (vpcId) {
    filters.push({ Name: 'vpc-id', Values: [vpcId] });
  }

  const result = await ec2.describeInstances({
    Filters: filters,
    MaxResults: 5,
  }).promise();

  const instances = (result.Reservations || []).flatMap((r) => r.Instances || []);
  if (instances.length === 0) return null;

  const instance = instances[0];
  const tags = {};
  for (const tag of instance.Tags || []) {
    tags[tag.Key.toLowerCase()] = tag.Value;
  }

  return {
    instanceId: instance.InstanceId,
    instanceName: tags.name || null,
    serviceName: tags.service || tags['service-name'] || tags.app || tags.application || null,
    team: tags.team || tags.owner || tags['cost-center'] || null,
    environment: tags.environment || tags.env || tags.stage || null,
    accountId: instance.OwnerId || null,
  };
}

async function lookupServiceCatalog(tenantId, sourceIp) {
  // Service catalog: tenant-uploaded mapping of IPs/CIDRs to services
  // Stored in DynamoDB as TENANT#<id> / CATALOG#<ip-or-cidr>
  const opts = { region: config.aws.region };
  if (config.aws.dynamoEndpoint) opts.endpoint = config.aws.dynamoEndpoint;
  const db = new AWS.DynamoDB.DocumentClient(opts);

  const result = await db.get({
    TableName: config.tables.tenants,
    Key: {
      PK: `TENANT#${tenantId}`,
      SK: `CATALOG#${sourceIp}`,
    },
  }).promise();

  return result.Item || null;
}

async function registerServiceMapping(tenantId, mapping) {
  const opts = { region: config.aws.region };
  if (config.aws.dynamoEndpoint) opts.endpoint = config.aws.dynamoEndpoint;
  const db = new AWS.DynamoDB.DocumentClient(opts);

  const item = {
    PK: `TENANT#${tenantId}`,
    SK: `CATALOG#${mapping.ipOrCidr}`,
    ipOrCidr: mapping.ipOrCidr,
    serviceName: mapping.serviceName,
    team: mapping.team || null,
    environment: mapping.environment || null,
    namespace: mapping.namespace || null,
    updatedAt: new Date().toISOString(),
  };

  await db.put({ TableName: config.tables.tenants, Item: item }).promise();
  // Invalidate cache entries for this IP
  enrichmentCache.delete(`${tenantId}:${mapping.ipOrCidr}`);
  return item;
}

async function getServiceCatalog(tenantId) {
  const opts = { region: config.aws.region };
  if (config.aws.dynamoEndpoint) opts.endpoint = config.aws.dynamoEndpoint;
  const db = new AWS.DynamoDB.DocumentClient(opts);

  const result = await db.query({
    TableName: config.tables.tenants,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'CATALOG#',
    },
  }).promise();

  return result.Items || [];
}

function clearCache() {
  enrichmentCache.clear();
}

module.exports = {
  enrichByIp,
  lookupEC2Instance,
  lookupServiceCatalog,
  registerServiceMapping,
  getServiceCatalog,
  clearCache,
};
