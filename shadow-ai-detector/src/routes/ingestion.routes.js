const express = require('express');
const router = express.Router();
const detection = require('../services/detection.service');

// Single event ingestion
// POST /api/v1/ingest/event
router.post('/event', async (req, res, next) => {
  try {
    const event = req.body;
    if (!event || typeof event !== 'object' || Array.isArray(event)) {
      return res.status(400).json({ error: 'Request body must be a JSON object' });
    }

    const result = await detection.processEvent(req.tenantId, event);
    res.status(result.action === 'finding_created' ? 201 : 200).json(result);
  } catch (err) {
    next(err);
  }
});

// Batch event ingestion (for high-throughput sources)
// POST /api/v1/ingest/batch
router.post('/batch', async (req, res, next) => {
  try {
    const { events } = req.body;
    if (!Array.isArray(events)) {
      return res.status(400).json({ error: 'Request body must contain an "events" array' });
    }
    if (events.length > 100) {
      return res.status(400).json({ error: 'Maximum 100 events per batch' });
    }

    const results = await detection.processBatch(req.tenantId, events);

    const summary = {
      total: results.length,
      findings_created: results.filter((r) => r.action === 'finding_created').length,
      allowlisted: results.filter((r) => r.action === 'allowlisted').length,
      skipped: results.filter((r) => r.action === 'skipped').length,
    };

    res.status(200).json({ summary, results });
  } catch (err) {
    next(err);
  }
});

// EventBridge-compatible ingestion
// POST /api/v1/ingest/eventbridge
// Accepts the native EventBridge event envelope from DNS Firewall
router.post('/eventbridge', async (req, res, next) => {
  try {
    const envelope = req.body;

    // EventBridge wraps the actual event in a 'detail' field
    const event = envelope.detail || envelope;

    // Normalise DNS Firewall alert fields
    const normalised = {
      queryName: event.queryName || event['query-name'],
      sourceIp: event.srcAddr || event['source-ip'] || event.sourceIp,
      vpcId: event.vpcId || event['vpc-id'],
      firewallRuleAction: event.firewallRuleAction || event['firewall-rule-action'],
      timestamp: envelope.time || event.timestamp || new Date().toISOString(),
      detectionMethod: 'dns_firewall',
    };

    const result = await detection.processEvent(req.tenantId, normalised);
    res.status(result.action === 'finding_created' ? 201 : 200).json(result);
  } catch (err) {
    next(err);
  }
});

module.exports = router;
