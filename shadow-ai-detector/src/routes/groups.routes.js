const express = require('express');
const router = express.Router();
const dedupService = require('../services/dedup.service');

// GET /api/v1/groups
// List detection groups (deduplicated view of findings)
router.get('/', async (req, res, next) => {
  try {
    const { status, providerId, limit, cursor } = req.query;
    const result = await dedupService.getGroups(req.tenantId, {
      status,
      providerId,
      limit: limit ? parseInt(limit, 10) : 50,
      lastKey: cursor,
    });
    res.json(result);
  } catch (err) {
    next(err);
  }
});

// GET /api/v1/groups/stats
// Aggregate stats across all groups
router.get('/stats', async (req, res, next) => {
  try {
    const stats = await dedupService.getGroupStats(req.tenantId);
    res.json(stats);
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/groups/:groupKey/acknowledge
// Acknowledge a detection group
router.post('/:groupKey/acknowledge', async (req, res, next) => {
  try {
    const { groupKey } = req.params;
    const { acknowledgedBy } = req.body;

    if (!acknowledgedBy) {
      return res.status(400).json({ error: 'acknowledgedBy is required' });
    }

    // groupKey uses :: separator, but URL-encode replaces it
    const decodedKey = decodeURIComponent(groupKey);
    const updated = await dedupService.acknowledgeGroup(
      req.tenantId, decodedKey, acknowledgedBy
    );

    if (!updated) {
      return res.status(404).json({ error: 'Group not found' });
    }

    res.json(updated);
  } catch (err) {
    next(err);
  }
});

module.exports = router;
