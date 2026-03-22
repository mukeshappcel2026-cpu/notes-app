const AWS = require('aws-sdk');
const { v4: uuidv4 } = require('uuid');
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

async function createFinding(tenantId, finding) {
  const db = getDocClient();
  const id = uuidv4();
  const now = new Date().toISOString();

  const item = {
    PK: `TENANT#${tenantId}`,
    SK: `FINDING#${now}#${id}`,
    findingId: id,
    tenantId,
    status: 'open',

    // Source info (who made the call)
    sourceIp: finding.sourceIp,
    sourceVpcId: finding.sourceVpcId || null,
    sourceEniId: finding.sourceEniId || null,

    // Detection info (what was detected)
    detectionMethod: finding.detectionMethod, // 'dns_firewall', 'nw_firewall_sni', 'flow_log'
    queryName: finding.queryName || null,
    sniValue: finding.sniValue || null,
    destinationIp: finding.destinationIp || null,

    // Matched provider
    providerId: finding.providerId,
    providerName: finding.providerName,
    riskTier: finding.riskTier,
    category: finding.category,

    // Timestamps
    detectedAt: finding.detectedAt || now,
    createdAt: now,
    acknowledgedAt: null,
    acknowledgedBy: null,
  };

  await db.put({ TableName: config.tables.findings, Item: item }).promise();
  return item;
}

async function getFindings(tenantId, { status, providerId, limit = 50, lastKey } = {}) {
  const db = getDocClient();
  const params = {
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'FINDING#',
    },
    ScanIndexForward: false, // newest first
    Limit: limit,
  };

  const filters = [];
  if (status) {
    filters.push('#status = :status');
    params.ExpressionAttributeValues[':status'] = status;
    params.ExpressionAttributeNames = { ...params.ExpressionAttributeNames, '#status': 'status' };
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
    findings: result.Items || [],
    nextKey: result.LastEvaluatedKey
      ? Buffer.from(JSON.stringify(result.LastEvaluatedKey)).toString('base64')
      : null,
  };
}

async function acknowledgeFinding(tenantId, findingId, acknowledgedBy) {
  const db = getDocClient();

  // Find the exact SK first
  const query = await db.query({
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    FilterExpression: 'findingId = :findingId',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'FINDING#',
      ':findingId': findingId,
    },
    Limit: 1,
  }).promise();

  if (!query.Items || query.Items.length === 0) {
    return null;
  }

  const item = query.Items[0];
  const result = await db.update({
    TableName: config.tables.findings,
    Key: { PK: item.PK, SK: item.SK },
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

async function getFindingStats(tenantId) {
  const db = getDocClient();

  const result = await db.query({
    TableName: config.tables.findings,
    KeyConditionExpression: 'PK = :pk AND begins_with(SK, :prefix)',
    ExpressionAttributeValues: {
      ':pk': `TENANT#${tenantId}`,
      ':prefix': 'FINDING#',
    },
    Select: 'ALL_ATTRIBUTES',
  }).promise();

  const items = result.Items || [];
  const stats = {
    total: items.length,
    open: 0,
    acknowledged: 0,
    byProvider: {},
    byRiskTier: { high: 0, medium: 0, low: 0 },
    byDetectionMethod: {},
    uniqueSourceIps: new Set(),
  };

  for (const item of items) {
    stats[item.status] = (stats[item.status] || 0) + 1;
    stats.byProvider[item.providerName] = (stats.byProvider[item.providerName] || 0) + 1;
    stats.byRiskTier[item.riskTier] = (stats.byRiskTier[item.riskTier] || 0) + 1;
    stats.byDetectionMethod[item.detectionMethod] = (stats.byDetectionMethod[item.detectionMethod] || 0) + 1;
    if (item.sourceIp) stats.uniqueSourceIps.add(item.sourceIp);
  }

  stats.uniqueSourceIps = stats.uniqueSourceIps.size;
  return stats;
}

module.exports = {
  createFinding,
  getFindings,
  acknowledgeFinding,
  getFindingStats,
};
