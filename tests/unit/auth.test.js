const { authenticate, _resetAuthClient } = require('../../src/middleware/auth');
const config = require('../../src/config');

// Mock google-auth-library
jest.mock('google-auth-library', () => ({
  OAuth2Client: jest.fn().mockImplementation(() => ({
    verifyIdToken: jest.fn(),
  })),
}));

const { OAuth2Client } = require('google-auth-library');

describe('Auth Middleware', () => {
  let req, res, next;
  let originalNodeEnv;
  let originalGoogleClientId;

  beforeEach(() => {
    originalNodeEnv = config.nodeEnv;
    originalGoogleClientId = config.googleClientId;
    _resetAuthClient();
    req = {
      headers: {},
    };
    res = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn().mockReturnThis(),
    };
    next = jest.fn();
    // Reset OAuth2Client mock
    OAuth2Client.mockClear();
  });

  afterEach(() => {
    config.nodeEnv = originalNodeEnv;
    config.googleClientId = originalGoogleClientId;
    _resetAuthClient();
  });

  describe('Test mode bypass', () => {
    test('should accept X-Test-User-Id header in test environment', async () => {
      config.nodeEnv = 'test';
      req.headers['x-test-user-id'] = 'test-user-123';

      await authenticate(req, res, next);

      expect(next).toHaveBeenCalled();
      expect(req.user).toEqual({
        userId: 'test-user-123',
        email: 'test@test.com',
        name: 'Test User',
      });
      expect(res.status).not.toHaveBeenCalled();
    });

    test('should fall through to auth check in test mode without test header', async () => {
      config.nodeEnv = 'test';
      // No X-Test-User-Id header, no Authorization header

      await authenticate(req, res, next);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Authorization header required' });
      expect(next).not.toHaveBeenCalled();
    });
  });

  describe('Authorization header validation', () => {
    beforeEach(() => {
      config.nodeEnv = 'production';
    });

    test('should return 401 when no Authorization header', async () => {
      await authenticate(req, res, next);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Authorization header required' });
      expect(next).not.toHaveBeenCalled();
    });

    test('should return 401 when Authorization header has wrong format', async () => {
      req.headers.authorization = 'Basic some-token';

      await authenticate(req, res, next);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Authorization header required' });
      expect(next).not.toHaveBeenCalled();
    });

    test('should return 500 when GOOGLE_CLIENT_ID not configured', async () => {
      config.googleClientId = null;
      req.headers.authorization = 'Bearer some-token';

      await authenticate(req, res, next);

      expect(res.status).toHaveBeenCalledWith(500);
      expect(res.json).toHaveBeenCalledWith({ error: 'Google Client ID not configured' });
      expect(next).not.toHaveBeenCalled();
    });

    test('should return 401 when token verification fails', async () => {
      config.googleClientId = 'test-client-id';
      _resetAuthClient();

      const mockVerify = jest.fn().mockRejectedValue(new Error('Invalid token'));
      OAuth2Client.mockImplementation(() => ({
        verifyIdToken: mockVerify,
      }));

      req.headers.authorization = 'Bearer invalid-token';

      await authenticate(req, res, next);

      expect(res.status).toHaveBeenCalledWith(401);
      expect(res.json).toHaveBeenCalledWith({ error: 'Invalid or expired token' });
      expect(next).not.toHaveBeenCalled();
    });

    test('should set req.user and call next on valid token', async () => {
      config.googleClientId = 'test-client-id';
      _resetAuthClient();

      const mockPayload = {
        sub: '117098219384756352841',
        email: 'user@gmail.com',
        name: 'Test User',
      };
      const mockVerify = jest.fn().mockResolvedValue({
        getPayload: () => mockPayload,
      });
      OAuth2Client.mockImplementation(() => ({
        verifyIdToken: mockVerify,
      }));

      req.headers.authorization = 'Bearer valid-token';

      await authenticate(req, res, next);

      expect(next).toHaveBeenCalled();
      expect(req.user).toEqual({
        userId: '117098219384756352841',
        email: 'user@gmail.com',
        name: 'Test User',
      });
      expect(res.status).not.toHaveBeenCalled();
    });
  });
});
