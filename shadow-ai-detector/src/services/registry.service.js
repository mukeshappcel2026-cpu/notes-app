const AWS = require('aws-sdk');
const config = require('../config');
const seedData = require('../data/ai-providers.json');

let docClient;

function getDocClient() {
  if (!docClient) {
    const opts = { region: config.aws.region };
    if (config.aws.dynamoEndpoint) opts.endpoint = config.aws.dynamoEndpoint;
    docClient = new AWS.DynamoDB.DocumentClient(opts);
  }
  return docClient;
}

// In-memory cache refreshed from DynamoDB periodically
let registryCache = null;
let cacheTimestamp = 0;
const CACHE_TTL_MS = 60 * 1000; // 1 minute

async function seedRegistry() {
  const db = getDocClient();
  const promises = seedData.providers.map((provider) =>
    db.put({
      TableName: config.tables.registry,
      Item: {
        PK: 'PROVIDER',
        SK: `PROVIDER#${provider.id}`,
        ...provider,
        updatedAt: new Date().toISOString(),
      },
      ConditionExpression: 'attribute_not_exists(PK)',
    }).promise().catch((err) => {
      if (err.code !== 'ConditionalCheckFailedException') throw err;
    })
  );
  await Promise.all(promises);
}

async function getAllProviders() {
  const now = Date.now();
  if (registryCache && now - cacheTimestamp < CACHE_TTL_MS) {
    return registryCache;
  }

  const db = getDocClient();
  const result = await db.query({
    TableName: config.tables.registry,
    KeyConditionExpression: 'PK = :pk',
    ExpressionAttributeValues: { ':pk': 'PROVIDER' },
  }).promise();

  registryCache = result.Items || [];
  cacheTimestamp = now;
  return registryCache;
}

async function getProvider(providerId) {
  const db = getDocClient();
  const result = await db.get({
    TableName: config.tables.registry,
    Key: { PK: 'PROVIDER', SK: `PROVIDER#${providerId}` },
  }).promise();
  return result.Item || null;
}

async function addCustomProvider(tenantId, provider) {
  const db = getDocClient();
  await db.put({
    TableName: config.tables.registry,
    Item: {
      PK: `TENANT#${tenantId}#PROVIDER`,
      SK: `PROVIDER#${provider.id}`,
      ...provider,
      addedBy: tenantId,
      updatedAt: new Date().toISOString(),
    },
  }).promise();
}

async function getProvidersForTenant(tenantId) {
  const db = getDocClient();

  // Get global providers + tenant-specific custom providers
  const [global, custom] = await Promise.all([
    getAllProviders(),
    db.query({
      TableName: config.tables.registry,
      KeyConditionExpression: 'PK = :pk',
      ExpressionAttributeValues: { ':pk': `TENANT#${tenantId}#PROVIDER` },
    }).promise(),
  ]);

  return [...global, ...(custom.Items || [])];
}

function invalidateCache() {
  registryCache = null;
  cacheTimestamp = 0;
}

async function upsertGlobalProvider(provider) {
  const db = getDocClient();
  await db.put({
    TableName: config.tables.registry,
    Item: {
      PK: 'PROVIDER',
      SK: `PROVIDER#${provider.id}`,
      ...provider,
      updatedAt: new Date().toISOString(),
    },
  }).promise();
  invalidateCache();
}

async function deleteGlobalProvider(providerId) {
  const db = getDocClient();
  await db.delete({
    TableName: config.tables.registry,
    Key: { PK: 'PROVIDER', SK: `PROVIDER#${providerId}` },
  }).promise();
  invalidateCache();
}

function buildDomainLookup(providers) {
  const lookup = new Map();
  for (const provider of providers) {
    if (!provider.domains) continue;
    for (const domain of provider.domains) {
      // Handle wildcard domains: *.openai.azure.com
      const normalised = domain.toLowerCase().replace(/^\*\./, '');
      lookup.set(normalised, provider);
    }
  }
  return lookup;
}

function matchDomain(queryName, domainLookup) {
  const normalised = queryName.toLowerCase().replace(/\.$/, ''); // strip trailing dot

  // Exact match
  if (domainLookup.has(normalised)) {
    return domainLookup.get(normalised);
  }

  // Wildcard match: check if any suffix matches
  // e.g., "my-deployment.openai.azure.com" should match "openai.azure.com"
  const parts = normalised.split('.');
  for (let i = 1; i < parts.length; i++) {
    const suffix = parts.slice(i).join('.');
    if (domainLookup.has(suffix)) {
      return domainLookup.get(suffix);
    }
  }

  return null;
}

module.exports = {
  seedRegistry,
  getAllProviders,
  getProvider,
  addCustomProvider,
  getProvidersForTenant,
  upsertGlobalProvider,
  deleteGlobalProvider,
  invalidateCache,
  buildDomainLookup,
  matchDomain,
};
