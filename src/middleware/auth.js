const { OAuth2Client } = require('google-auth-library');
const config = require('../config');

// Lazy-initialized OAuth2 client (same pattern as note.service.js DynamoDB client)
let client;
function getClient() {
  if (!client && config.googleClientId) {
    client = new OAuth2Client(config.googleClientId);
  }
  return client;
}

/**
 * Authentication middleware - verifies Google ID token from Authorization header.
 * In test mode (NODE_ENV=test), accepts X-Test-User-Id header as bypass.
 */
async function authenticate(req, res, next) {
  // Test bypass: accept X-Test-User-Id header in test environment
  if (config.nodeEnv === 'test') {
    const testUserId = req.headers['x-test-user-id'];
    if (testUserId) {
      req.user = { userId: testUserId, email: 'test@test.com', name: 'Test User' };
      return next();
    }
    // In test mode without test header, still require auth (fall through)
  }

  const authHeader = req.headers.authorization;
  if (!authHeader || !authHeader.startsWith('Bearer ')) {
    return res.status(401).json({ error: 'Authorization header required' });
  }

  const token = authHeader.split(' ')[1];

  const oauthClient = getClient();
  if (!oauthClient) {
    return res.status(500).json({ error: 'Google Client ID not configured' });
  }

  try {
    const ticket = await oauthClient.verifyIdToken({
      idToken: token,
      audience: config.googleClientId,
    });

    const payload = ticket.getPayload();
    req.user = {
      userId: payload.sub,
      email: payload.email,
      name: payload.name,
    };
    next();
  } catch (err) {
    console.error('Token verification failed:', err.message);
    return res.status(401).json({ error: 'Invalid or expired token' });
  }
}

// Reset client for testing
function _resetAuthClient() {
  client = null;
}

module.exports = { authenticate, _resetAuthClient };
