const registryService = require('./registry.service');
const allowlistService = require('./allowlist.service');
const findingsService = require('./findings.service');
const alertingService = require('./alerting.service');

async function processEvent(tenantId, event) {
  // Step 1: Load provider registry and tenant allowlist
  const [providers, allowlist] = await Promise.all([
    registryService.getProvidersForTenant(tenantId),
    allowlistService.getAllowlist(tenantId),
  ]);

  const domainLookup = registryService.buildDomainLookup(providers);

  // Step 2: Extract the domain to match based on event type
  const domainToMatch = extractDomain(event);
  if (!domainToMatch) {
    return { action: 'skipped', reason: 'no_matchable_domain' };
  }

  // Step 3: Match against provider registry
  const matchedProvider = registryService.matchDomain(domainToMatch, domainLookup);
  if (!matchedProvider) {
    return { action: 'skipped', reason: 'no_provider_match' };
  }

  // Step 4: Check allowlist (is this a known gateway?)
  const sourceIp = event.sourceIp || event.srcAddr;
  const allowCheck = allowlistService.isAllowlisted(sourceIp, allowlist);
  if (allowCheck.allowed) {
    return {
      action: 'allowlisted',
      reason: allowCheck.reason,
      provider: matchedProvider.name,
      sourceIp,
    };
  }

  // Step 5: Create finding — this is a shadow AI detection!
  const finding = await findingsService.createFinding(tenantId, {
    sourceIp,
    sourceVpcId: event.vpcId || event.sourceVpcId || null,
    sourceEniId: event.eniId || null,
    detectionMethod: event.detectionMethod || classifyDetectionMethod(event),
    queryName: event.queryName || null,
    sniValue: event.sniValue || null,
    destinationIp: event.destinationIp || event.dstAddr || null,
    providerId: matchedProvider.id,
    providerName: matchedProvider.name,
    riskTier: matchedProvider.riskTier,
    category: matchedProvider.category,
    detectedAt: event.timestamp || new Date().toISOString(),
  });

  // Step 6: Fire alert (non-blocking)
  alertingService.sendAlert(tenantId, finding).catch((err) => {
    console.error('Alert delivery failed (non-fatal):', err.message);
  });

  return {
    action: 'finding_created',
    findingId: finding.findingId,
    provider: matchedProvider.name,
    riskTier: matchedProvider.riskTier,
    sourceIp,
  };
}

async function processBatch(tenantId, events) {
  const results = [];
  // Load shared data once for the batch
  const [providers, allowlist] = await Promise.all([
    registryService.getProvidersForTenant(tenantId),
    allowlistService.getAllowlist(tenantId),
  ]);

  const domainLookup = registryService.buildDomainLookup(providers);

  for (const event of events) {
    const domainToMatch = extractDomain(event);
    if (!domainToMatch) {
      results.push({ action: 'skipped', reason: 'no_matchable_domain' });
      continue;
    }

    const matchedProvider = registryService.matchDomain(domainToMatch, domainLookup);
    if (!matchedProvider) {
      results.push({ action: 'skipped', reason: 'no_provider_match' });
      continue;
    }

    const sourceIp = event.sourceIp || event.srcAddr;
    const allowCheck = allowlistService.isAllowlisted(sourceIp, allowlist);
    if (allowCheck.allowed) {
      results.push({ action: 'allowlisted', provider: matchedProvider.name, sourceIp });
      continue;
    }

    const finding = await findingsService.createFinding(tenantId, {
      sourceIp,
      sourceVpcId: event.vpcId || event.sourceVpcId || null,
      sourceEniId: event.eniId || null,
      detectionMethod: event.detectionMethod || classifyDetectionMethod(event),
      queryName: event.queryName || null,
      sniValue: event.sniValue || null,
      destinationIp: event.destinationIp || event.dstAddr || null,
      providerId: matchedProvider.id,
      providerName: matchedProvider.name,
      riskTier: matchedProvider.riskTier,
      category: matchedProvider.category,
      detectedAt: event.timestamp || new Date().toISOString(),
    });

    alertingService.sendAlert(tenantId, finding).catch(() => {});
    results.push({ action: 'finding_created', findingId: finding.findingId, provider: matchedProvider.name });
  }

  return results;
}

function extractDomain(event) {
  // DNS Firewall alert: has queryName
  if (event.queryName) return event.queryName;

  // Network Firewall SNI alert: has sniValue or tls.sni
  if (event.sniValue) return event.sniValue;
  if (event.tls && event.tls.sni) return event.tls.sni;

  // Suricata alert format
  if (event.alert && event.alert.signature) {
    const sniMatch = event.alert.signature.match(/SNI:\s*(\S+)/i);
    if (sniMatch) return sniMatch[1];
  }

  return null;
}

function classifyDetectionMethod(event) {
  if (event.queryName && event.firewallRuleAction) return 'dns_firewall';
  if (event.sniValue || (event.tls && event.tls.sni)) return 'nw_firewall_sni';
  if (event.srcAddr && event.dstAddr && event.protocol) return 'flow_log';
  return 'unknown';
}

module.exports = {
  processEvent,
  processBatch,
  extractDomain,
  classifyDetectionMethod,
};
