const registryService = require('./registry.service');
const allowlistService = require('./allowlist.service');
const findingsService = require('./findings.service');
const alertingService = require('./alerting.service');
const enrichmentService = require('./enrichment.service');
const dedupService = require('./dedup.service');

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

  // Step 5: Enrich with service/team attribution (best-effort)
  let enrichment = {};
  try {
    enrichment = await enrichmentService.enrichByIp(
      tenantId, sourceIp, event.vpcId || event.sourceVpcId
    );
  } catch (err) {
    console.error('Enrichment failed (non-fatal):', err.message);
  }

  // Step 6: Deduplication — check for active group
  const detectionMethod = event.detectionMethod || classifyDetectionMethod(event);
  const activeGroup = await dedupService.findActiveGroup(tenantId, sourceIp, matchedProvider.id);

  if (activeGroup) {
    // Existing group — just increment the counter
    const updated = await dedupService.incrementGroup(tenantId, activeGroup, {
      detectionMethod,
      queryName: event.queryName || null,
      sniValue: event.sniValue || null,
      serviceName: enrichment.serviceName,
      team: enrichment.team,
    });

    return {
      action: 'grouped',
      groupKey: activeGroup.groupKey,
      eventCount: updated.eventCount,
      provider: matchedProvider.name,
      sourceIp,
      serviceName: updated.serviceName || null,
      team: updated.team || null,
    };
  }

  // Step 7: No active group — create finding + new group
  const findingData = {
    sourceIp,
    sourceVpcId: event.vpcId || event.sourceVpcId || null,
    sourceEniId: event.eniId || null,
    detectionMethod,
    queryName: event.queryName || null,
    sniValue: event.sniValue || null,
    destinationIp: event.destinationIp || event.dstAddr || null,
    providerId: matchedProvider.id,
    providerName: matchedProvider.name,
    riskTier: matchedProvider.riskTier,
    category: matchedProvider.category,
    detectedAt: event.timestamp || new Date().toISOString(),
    // Enrichment fields
    instanceId: enrichment.instanceId || null,
    instanceName: enrichment.instanceName || null,
    serviceName: enrichment.serviceName || null,
    team: enrichment.team || null,
    environment: enrichment.environment || null,
    enrichmentSource: enrichment.enrichmentSource || 'none',
  };

  const finding = await findingsService.createFinding(tenantId, findingData);

  // Create dedup group anchored to this finding
  await dedupService.createGroup(tenantId, {
    ...findingData,
    findingId: finding.findingId,
  });

  // Step 8: Fire alert (non-blocking)
  alertingService.sendAlert(tenantId, finding).catch((err) => {
    console.error('Alert delivery failed (non-fatal):', err.message);
  });

  return {
    action: 'finding_created',
    findingId: finding.findingId,
    provider: matchedProvider.name,
    riskTier: matchedProvider.riskTier,
    sourceIp,
    serviceName: enrichment.serviceName || null,
    team: enrichment.team || null,
  };
}

async function processBatch(tenantId, events) {
  const results = [];
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

    // Enrich (best-effort)
    let enrichment = {};
    try {
      enrichment = await enrichmentService.enrichByIp(
        tenantId, sourceIp, event.vpcId || event.sourceVpcId
      );
    } catch (_) { /* non-fatal */ }

    const detectionMethod = event.detectionMethod || classifyDetectionMethod(event);

    // Dedup check
    const activeGroup = await dedupService.findActiveGroup(tenantId, sourceIp, matchedProvider.id);
    if (activeGroup) {
      const updated = await dedupService.incrementGroup(tenantId, activeGroup, {
        detectionMethod,
        queryName: event.queryName || null,
        sniValue: event.sniValue || null,
        serviceName: enrichment.serviceName,
        team: enrichment.team,
      });
      results.push({
        action: 'grouped',
        groupKey: activeGroup.groupKey,
        eventCount: updated.eventCount,
        provider: matchedProvider.name,
      });
      continue;
    }

    const findingData = {
      sourceIp,
      sourceVpcId: event.vpcId || event.sourceVpcId || null,
      sourceEniId: event.eniId || null,
      detectionMethod,
      queryName: event.queryName || null,
      sniValue: event.sniValue || null,
      destinationIp: event.destinationIp || event.dstAddr || null,
      providerId: matchedProvider.id,
      providerName: matchedProvider.name,
      riskTier: matchedProvider.riskTier,
      category: matchedProvider.category,
      detectedAt: event.timestamp || new Date().toISOString(),
      instanceId: enrichment.instanceId || null,
      instanceName: enrichment.instanceName || null,
      serviceName: enrichment.serviceName || null,
      team: enrichment.team || null,
      environment: enrichment.environment || null,
      enrichmentSource: enrichment.enrichmentSource || 'none',
    };

    const finding = await findingsService.createFinding(tenantId, findingData);
    await dedupService.createGroup(tenantId, { ...findingData, findingId: finding.findingId });

    alertingService.sendAlert(tenantId, finding).catch(() => {});
    results.push({
      action: 'finding_created',
      findingId: finding.findingId,
      provider: matchedProvider.name,
      serviceName: enrichment.serviceName || null,
    });
  }

  return results;
}

function extractDomain(event) {
  if (event.queryName) return event.queryName;
  if (event.sniValue) return event.sniValue;
  if (event.tls && event.tls.sni) return event.tls.sni;
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
