const express = require('express');
const router = express.Router();
const enrichmentService = require('../services/enrichment.service');

// GET /api/v1/enrichment/catalog
// List all service catalog mappings for this tenant
router.get('/catalog', async (req, res, next) => {
  try {
    const entries = await enrichmentService.getServiceCatalog(req.tenantId);
    res.json({ entries, count: entries.length });
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/enrichment/catalog
// Register an IP → service mapping
router.post('/catalog', async (req, res, next) => {
  try {
    const { ipOrCidr, serviceName, team, environment, namespace } = req.body;

    if (!ipOrCidr || !serviceName) {
      return res.status(400).json({ error: 'ipOrCidr and serviceName are required' });
    }

    const entry = await enrichmentService.registerServiceMapping(req.tenantId, {
      ipOrCidr,
      serviceName,
      team,
      environment,
      namespace,
    });

    res.status(201).json(entry);
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/enrichment/catalog/bulk
// Bulk import service catalog entries
router.post('/catalog/bulk', async (req, res, next) => {
  try {
    const { entries } = req.body;

    if (!Array.isArray(entries) || entries.length === 0) {
      return res.status(400).json({ error: 'entries must be a non-empty array' });
    }
    if (entries.length > 500) {
      return res.status(400).json({ error: 'Maximum 500 entries per bulk import' });
    }

    const results = { created: 0, failed: 0, errors: [] };

    for (const entry of entries) {
      try {
        if (!entry.ipOrCidr || !entry.serviceName) {
          results.failed++;
          results.errors.push({ ipOrCidr: entry.ipOrCidr, error: 'missing ipOrCidr or serviceName' });
          continue;
        }
        await enrichmentService.registerServiceMapping(req.tenantId, entry);
        results.created++;
      } catch (err) {
        results.failed++;
        results.errors.push({ ipOrCidr: entry.ipOrCidr, error: err.message });
      }
    }

    res.status(200).json(results);
  } catch (err) {
    next(err);
  }
});

// GET /api/v1/enrichment/lookup/:ip
// Enrich a single IP (for debugging / manual lookup)
router.get('/lookup/:ip', async (req, res, next) => {
  try {
    const enrichment = await enrichmentService.enrichByIp(
      req.tenantId,
      req.params.ip,
      req.query.vpcId || null
    );
    res.json(enrichment);
  } catch (err) {
    next(err);
  }
});

module.exports = router;
