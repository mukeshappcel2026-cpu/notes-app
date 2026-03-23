const config = require('../config');

function tenantAuth(req, res, next) {
  // In test mode, accept X-Tenant-Id header directly
  if (config.env === 'test') {
    const testTenantId = req.headers['x-tenant-id'];
    if (testTenantId) {
      req.tenantId = testTenantId;
      return next();
    }
  }

  const apiKey = req.headers['x-api-key'];
  if (!apiKey) {
    return res.status(401).json({ error: 'Missing X-API-Key header' });
  }

  // API key format: sad_<tenantId>_<secret>
  // In production, validate against DynamoDB tenants table.
  // For MVP, extract tenantId from key format and trust it.
  const parts = apiKey.split('_');
  if (parts.length < 3 || parts[0] !== 'sad') {
    return res.status(401).json({ error: 'Invalid API key format' });
  }

  req.tenantId = parts[1];
  req.apiKey = apiKey;
  next();
}

module.exports = { tenantAuth };
