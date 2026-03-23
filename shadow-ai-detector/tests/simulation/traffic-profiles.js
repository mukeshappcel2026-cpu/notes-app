/**
 * Shadow AI Traffic Simulator
 *
 * Generates realistic event streams that mimic what DNS Firewall and
 * Network Firewall would produce when cloud workloads call AI providers
 * directly (bypassing the approved AI gateway).
 *
 * Simulates 6 different "shadow AI apps":
 *   1. payments-api     → calling OpenAI directly (HIGH risk — PII exposure)
 *   2. search-indexer   → calling Anthropic directly
 *   3. marketing-bot    → calling DeepSeek (HIGH risk — data sovereignty)
 *   4. ml-pipeline      → calling Hugging Face (legitimate, should be allowlisted)
 *   5. internal-tool    → calling Gemini via Azure OpenAI
 *   6. rogue-developer  → calling 3 providers from same IP (noisy)
 */

// Simulated services with realistic VPC topology
const SHADOW_APPS = [
  {
    id: 'payments-api',
    description: 'Payment service calling OpenAI to summarise transaction disputes',
    sourceIp: '10.0.5.42',
    vpcId: 'vpc-prod-001',
    eniId: 'eni-0a1b2c3d4e5f',
    team: 'payments',
    environment: 'production',
    events: [
      // DNS Firewall alert format (Route53 Resolver)
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:00:00Z',
      },
      // Repeated calls (should be deduped into same group)
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:05:00Z',
      },
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:10:00Z',
      },
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:15:00Z',
      },
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.5.42',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:20:00Z',
      },
    ],
  },
  {
    id: 'search-indexer',
    description: 'Search service calling Anthropic to generate embeddings',
    sourceIp: '10.0.6.10',
    vpcId: 'vpc-prod-001',
    team: 'search',
    environment: 'production',
    events: [
      {
        queryName: 'api.anthropic.com',
        sourceIp: '10.0.6.10',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:02:00Z',
      },
      {
        queryName: 'api.anthropic.com',
        sourceIp: '10.0.6.10',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T10:12:00Z',
      },
    ],
  },
  {
    id: 'marketing-bot',
    description: 'Marketing automation calling DeepSeek (data sovereignty concern)',
    sourceIp: '10.0.8.55',
    vpcId: 'vpc-prod-002',
    team: 'marketing',
    environment: 'production',
    events: [
      // Network Firewall SNI format (Suricata-style)
      {
        sniValue: 'api.deepseek.com',
        sourceIp: '10.0.8.55',
        destinationIp: '203.0.113.50',
        vpcId: 'vpc-prod-002',
        timestamp: '2026-03-22T11:00:00Z',
      },
      {
        sniValue: 'chat.deepseek.com',
        sourceIp: '10.0.8.55',
        destinationIp: '203.0.113.51',
        vpcId: 'vpc-prod-002',
        timestamp: '2026-03-22T11:05:00Z',
      },
    ],
  },
  {
    id: 'ml-pipeline',
    description: 'ML team calling Hugging Face (LEGITIMATE — should be allowlisted)',
    sourceIp: '10.0.3.10', // This is the AI gateway IP
    vpcId: 'vpc-prod-001',
    team: 'ml-platform',
    environment: 'production',
    isGateway: true, // Should be in the allowlist
    events: [
      {
        queryName: 'api-inference.huggingface.co',
        sourceIp: '10.0.3.10',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T09:00:00Z',
      },
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.3.10',
        vpcId: 'vpc-prod-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T09:05:00Z',
      },
    ],
  },
  {
    id: 'internal-tool',
    description: 'Internal admin tool calling Azure OpenAI (wildcard subdomain)',
    sourceIp: '10.0.7.22',
    vpcId: 'vpc-staging-001',
    team: 'platform-eng',
    environment: 'staging',
    events: [
      {
        queryName: 'acme-gpt4.openai.azure.com',
        sourceIp: '10.0.7.22',
        vpcId: 'vpc-staging-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T14:00:00Z',
      },
    ],
  },
  {
    id: 'rogue-developer',
    description: 'Single dev box calling 3 different AI providers',
    sourceIp: '10.0.9.99',
    vpcId: 'vpc-dev-001',
    team: 'unknown',
    environment: 'development',
    events: [
      {
        queryName: 'api.openai.com',
        sourceIp: '10.0.9.99',
        vpcId: 'vpc-dev-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T15:00:00Z',
      },
      {
        queryName: 'api.anthropic.com',
        sourceIp: '10.0.9.99',
        vpcId: 'vpc-dev-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T15:01:00Z',
      },
      {
        queryName: 'api.mistral.ai',
        sourceIp: '10.0.9.99',
        vpcId: 'vpc-dev-001',
        firewallRuleAction: 'ALERT',
        timestamp: '2026-03-22T15:02:00Z',
      },
      // Also detected via Network Firewall SNI (second detection method)
      {
        sniValue: 'api.openai.com',
        sourceIp: '10.0.9.99',
        destinationIp: '104.18.6.192',
        vpcId: 'vpc-dev-001',
        timestamp: '2026-03-22T15:03:00Z',
      },
    ],
  },
];

// EventBridge envelope format (what actually arrives from customer AWS account)
function wrapInEventBridge(event) {
  return {
    version: '0',
    id: `evt-${Math.random().toString(36).slice(2, 10)}`,
    source: 'aws.route53resolver',
    'detail-type': 'DNS Firewall Alert',
    account: '123456789012',
    region: 'us-east-1',
    time: event.timestamp,
    detail: {
      'query-name': event.queryName,
      'source-ip': event.sourceIp,
      'vpc-id': event.vpcId,
      'firewall-rule-action': event.firewallRuleAction || 'ALERT',
    },
  };
}

module.exports = {
  SHADOW_APPS,
  wrapInEventBridge,
};
