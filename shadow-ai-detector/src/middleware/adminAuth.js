const config = require('../config');

function adminAuth(req, res, next) {
  // In test mode, accept X-Admin header directly
  if (config.env === 'test' && req.headers['x-admin'] === 'true') {
    return next();
  }

  const apiKey = req.headers['x-admin-key'];
  if (!apiKey) {
    return res.status(401).json({ error: 'Missing X-Admin-Key header' });
  }

  if (!config.admin.apiKey) {
    return res.status(503).json({ error: 'Admin API not configured' });
  }

  if (apiKey !== config.admin.apiKey) {
    return res.status(403).json({ error: 'Invalid admin key' });
  }

  next();
}

module.exports = { adminAuth };
