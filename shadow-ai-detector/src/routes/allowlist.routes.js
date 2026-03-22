const express = require('express');
const router = express.Router();
const allowlistService = require('../services/allowlist.service');

// GET /api/v1/allowlist
router.get('/', async (req, res, next) => {
  try {
    const entries = await allowlistService.getAllowlist(req.tenantId);
    res.json({ entries, count: entries.length });
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/allowlist
// Add a gateway IP or source that should NOT trigger findings
router.post('/', async (req, res, next) => {
  try {
    const { type, value, reason } = req.body;

    const validTypes = ['source_ip', 'source_cidr', 'source_eni'];
    if (!type || !validTypes.includes(type)) {
      return res.status(400).json({
        error: `type must be one of: ${validTypes.join(', ')}`,
      });
    }
    if (!value) {
      return res.status(400).json({ error: 'value is required' });
    }

    const entry = await allowlistService.addAllowlistEntry(req.tenantId, {
      type,
      value,
      reason: reason || '',
      createdBy: req.tenantId,
    });

    res.status(201).json(entry);
  } catch (err) {
    next(err);
  }
});

// DELETE /api/v1/allowlist/:type/:value
router.delete('/:type/:value', async (req, res, next) => {
  try {
    const { type, value } = req.params;
    await allowlistService.removeAllowlistEntry(req.tenantId, type, value);
    res.status(204).end();
  } catch (err) {
    next(err);
  }
});

module.exports = router;
