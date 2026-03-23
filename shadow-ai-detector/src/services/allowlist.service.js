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

// In-memory per-tenant allowlist cache
const allowlistCache = new Map();
const CACHE_TTL_MS = 60 * 1000;

async function getAllowlist(tenantId) {
  const cached = allowlistCache.get(tenantId);
  if (cached && Date.now() - cached.timestamp < CACHE_TTL_MS) {
    return cached.entries;
  }

  const db = getDocClient();
  const result = await db.query({
    TableName: config.tables.allowlist,
    KeyConditionExpression: 'PK = :pk',
    ExpressionAttributeValues: { ':pk': `TENANT#${tenantId}` },
  }).promise();

  const entries = result.Items || [];
  allowlistCache.set(tenantId, { entries, timestamp: Date.now() });
  return entries;
}

async function addAllowlistEntry(tenantId, entry) {
  const db = getDocClient();
  const item = {
    PK: `TENANT#${tenantId}`,
    SK: `ALLOW#${entry.type}#${entry.value}`,
    type: entry.type, // 'source_ip', 'source_cidr', 'source_eni', 'domain'
    value: entry.value,
    reason: entry.reason || '',
    createdBy: entry.createdBy || 'system',
    createdAt: new Date().toISOString(),
  };

  await db.put({ TableName: config.tables.allowlist, Item: item }).promise();
  allowlistCache.delete(tenantId); // invalidate cache
  return item;
}

async function removeAllowlistEntry(tenantId, type, value) {
  const db = getDocClient();
  await db.delete({
    TableName: config.tables.allowlist,
    Key: { PK: `TENANT#${tenantId}`, SK: `ALLOW#${type}#${value}` },
  }).promise();
  allowlistCache.delete(tenantId);
}

function isAllowlisted(sourceIp, allowlistEntries) {
  for (const entry of allowlistEntries) {
    if (entry.type === 'source_ip' && entry.value === sourceIp) {
      return { allowed: true, reason: entry.reason };
    }
    // CIDR matching would go here (needs a library like ip-cidr)
    // For MVP, we support exact IP matching only
  }
  return { allowed: false };
}

module.exports = {
  getAllowlist,
  addAllowlistEntry,
  removeAllowlistEntry,
  isAllowlisted,
};
