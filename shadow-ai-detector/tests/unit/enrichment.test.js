const { clearCache } = require('../../src/services/enrichment.service');

describe('Enrichment Service', () => {
  afterEach(() => {
    clearCache();
  });

  describe('cache management', () => {
    it('clearCache does not throw', () => {
      expect(() => clearCache()).not.toThrow();
    });

    it('clearCache can be called multiple times', () => {
      clearCache();
      clearCache();
      clearCache();
    });
  });
});
