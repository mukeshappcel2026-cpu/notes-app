const express = require('express');
const router = express.Router();
const registryService = require('../services/registry.service');

// GET /api/v1/admin/providers — list all global providers
router.get('/providers', async (req, res, next) => {
  try {
    const providers = await registryService.getAllProviders();
    res.json({ providers, count: providers.length });
  } catch (err) {
    next(err);
  }
});

// PUT /api/v1/admin/providers/:providerId — add or update a global provider
router.put('/providers/:providerId', async (req, res, next) => {
  try {
    const { providerId } = req.params;
    const { name, domains, category, riskTier, sniPatterns } = req.body;

    if (!name || !domains || !Array.isArray(domains) || domains.length === 0) {
      return res.status(400).json({
        error: 'name and domains (non-empty array) are required',
      });
    }

    const existing = await registryService.getProvider(providerId);

    await registryService.upsertGlobalProvider({
      id: providerId,
      name,
      domains,
      category: category || 'custom',
      riskTier: riskTier || 'medium',
      sniPatterns: sniPatterns || [],
    });

    const status = existing ? 200 : 201;
    const message = existing ? 'Provider updated' : 'Provider created';
    res.status(status).json({ message, id: providerId });
  } catch (err) {
    next(err);
  }
});

// DELETE /api/v1/admin/providers/:providerId — remove a global provider
router.delete('/providers/:providerId', async (req, res, next) => {
  try {
    const { providerId } = req.params;
    const existing = await registryService.getProvider(providerId);
    if (!existing) {
      return res.status(404).json({ error: 'Provider not found' });
    }

    await registryService.deleteGlobalProvider(providerId);
    res.json({ message: 'Provider deleted', id: providerId });
  } catch (err) {
    next(err);
  }
});

module.exports = router;
