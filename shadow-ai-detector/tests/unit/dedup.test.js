const { buildGroupKey, DEFAULT_GROUP_WINDOW_MS } = require('../../src/services/dedup.service');

describe('Dedup Service', () => {
  describe('buildGroupKey', () => {
    it('creates a key from sourceIp and providerId', () => {
      const key = buildGroupKey('10.0.5.42', 'openai');
      expect(key).toBe('10.0.5.42::openai');
    });

    it('creates distinct keys for different IPs same provider', () => {
      const key1 = buildGroupKey('10.0.5.42', 'openai');
      const key2 = buildGroupKey('10.0.5.43', 'openai');
      expect(key1).not.toBe(key2);
    });

    it('creates distinct keys for same IP different providers', () => {
      const key1 = buildGroupKey('10.0.5.42', 'openai');
      const key2 = buildGroupKey('10.0.5.42', 'anthropic');
      expect(key1).not.toBe(key2);
    });
  });

  describe('DEFAULT_GROUP_WINDOW_MS', () => {
    it('is 1 hour', () => {
      expect(DEFAULT_GROUP_WINDOW_MS).toBe(60 * 60 * 1000);
    });
  });
});
