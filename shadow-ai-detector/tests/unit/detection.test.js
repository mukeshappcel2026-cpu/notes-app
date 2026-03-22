const { extractDomain, classifyDetectionMethod } = require('../../src/services/detection.service');
const { matchDomain, buildDomainLookup } = require('../../src/services/registry.service');

describe('Detection Engine', () => {
  describe('extractDomain', () => {
    it('extracts queryName from DNS Firewall alerts', () => {
      const event = {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        firewallRuleAction: 'ALERT',
      };
      expect(extractDomain(event)).toBe('api.openai.com');
    });

    it('extracts sniValue from Network Firewall alerts', () => {
      const event = {
        sniValue: 'api.anthropic.com',
        sourceIp: '10.0.5.42',
      };
      expect(extractDomain(event)).toBe('api.anthropic.com');
    });

    it('extracts nested tls.sni', () => {
      const event = {
        tls: { sni: 'api.mistral.ai' },
        sourceIp: '10.0.5.42',
      };
      expect(extractDomain(event)).toBe('api.mistral.ai');
    });

    it('returns null when no domain can be extracted', () => {
      const event = { srcAddr: '10.0.5.42', dstAddr: '104.18.6.192' };
      expect(extractDomain(event)).toBeNull();
    });

    it('prioritises queryName over sniValue', () => {
      const event = {
        queryName: 'api.openai.com',
        sniValue: 'something-else.com',
      };
      expect(extractDomain(event)).toBe('api.openai.com');
    });
  });

  describe('classifyDetectionMethod', () => {
    it('classifies DNS Firewall events', () => {
      expect(classifyDetectionMethod({
        queryName: 'api.openai.com',
        firewallRuleAction: 'ALERT',
      })).toBe('dns_firewall');
    });

    it('classifies Network Firewall SNI events', () => {
      expect(classifyDetectionMethod({
        sniValue: 'api.openai.com',
      })).toBe('nw_firewall_sni');
    });

    it('classifies flow log events', () => {
      expect(classifyDetectionMethod({
        srcAddr: '10.0.5.42',
        dstAddr: '104.18.6.192',
        protocol: 6,
      })).toBe('flow_log');
    });

    it('returns unknown for unrecognised formats', () => {
      expect(classifyDetectionMethod({ foo: 'bar' })).toBe('unknown');
    });
  });

  describe('matchDomain', () => {
    const providers = [
      {
        id: 'openai',
        name: 'OpenAI',
        domains: ['api.openai.com', 'chatgpt.com'],
        riskTier: 'high',
      },
      {
        id: 'anthropic',
        name: 'Anthropic',
        domains: ['api.anthropic.com', 'claude.ai'],
        riskTier: 'high',
      },
      {
        id: 'azure-openai',
        name: 'Azure OpenAI',
        domains: ['*.openai.azure.com'],
        riskTier: 'high',
      },
    ];

    let lookup;

    beforeAll(() => {
      lookup = buildDomainLookup(providers);
    });

    it('matches exact domain', () => {
      const match = matchDomain('api.openai.com', lookup);
      expect(match).not.toBeNull();
      expect(match.id).toBe('openai');
    });

    it('matches domain with trailing dot (DNS format)', () => {
      const match = matchDomain('api.openai.com.', lookup);
      expect(match).not.toBeNull();
      expect(match.id).toBe('openai');
    });

    it('matches case-insensitively', () => {
      const match = matchDomain('API.OpenAI.COM', lookup);
      expect(match).not.toBeNull();
      expect(match.id).toBe('openai');
    });

    it('matches wildcard subdomains (Azure OpenAI)', () => {
      const match = matchDomain('my-deployment.openai.azure.com', lookup);
      expect(match).not.toBeNull();
      expect(match.id).toBe('azure-openai');
    });

    it('returns null for non-AI domains', () => {
      const match = matchDomain('google.com', lookup);
      expect(match).toBeNull();
    });

    it('returns null for partial matches', () => {
      const match = matchDomain('notapi.openai.com.evil.com', lookup);
      expect(match).toBeNull();
    });
  });
});
