const { isAllowlisted } = require('../../src/services/allowlist.service');

describe('Allowlist Service', () => {
  describe('isAllowlisted', () => {
    const allowlist = [
      { type: 'source_ip', value: '10.0.3.10', reason: 'AI Gateway primary' },
      { type: 'source_ip', value: '10.0.3.11', reason: 'AI Gateway secondary' },
    ];

    it('returns allowed=true for allowlisted IPs', () => {
      const result = isAllowlisted('10.0.3.10', allowlist);
      expect(result.allowed).toBe(true);
      expect(result.reason).toBe('AI Gateway primary');
    });

    it('returns allowed=false for non-allowlisted IPs', () => {
      const result = isAllowlisted('10.0.5.42', allowlist);
      expect(result.allowed).toBe(false);
    });

    it('returns allowed=false for empty allowlist', () => {
      const result = isAllowlisted('10.0.3.10', []);
      expect(result.allowed).toBe(false);
    });
  });
});
