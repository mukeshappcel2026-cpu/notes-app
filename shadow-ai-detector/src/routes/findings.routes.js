const express = require('express');
const router = express.Router();
const findingsService = require('../services/findings.service');

// GET /api/v1/findings
router.get('/', async (req, res, next) => {
  try {
    const { status, providerId, team, serviceName, limit, cursor } = req.query;
    const result = await findingsService.getFindings(req.tenantId, {
      status,
      providerId,
      team,
      serviceName,
      limit: limit ? parseInt(limit, 10) : 50,
      lastKey: cursor,
    });
    res.json(result);
  } catch (err) {
    next(err);
  }
});

// GET /api/v1/findings/stats
router.get('/stats', async (req, res, next) => {
  try {
    const stats = await findingsService.getFindingStats(req.tenantId);
    res.json(stats);
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/findings/:findingId/acknowledge
router.post('/:findingId/acknowledge', async (req, res, next) => {
  try {
    const { findingId } = req.params;
    const { acknowledgedBy } = req.body;

    if (!acknowledgedBy) {
      return res.status(400).json({ error: 'acknowledgedBy is required' });
    }

    const updated = await findingsService.acknowledgeFinding(
      req.tenantId, findingId, acknowledgedBy
    );

    if (!updated) {
      return res.status(404).json({ error: 'Finding not found' });
    }

    res.json(updated);
  } catch (err) {
    next(err);
  }
});

module.exports = router;
