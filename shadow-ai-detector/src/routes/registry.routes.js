const express = require('express');
const router = express.Router();
const registryService = require('../services/registry.service');

// GET /api/v1/registry/providers
router.get('/providers', async (req, res, next) => {
  try {
    const providers = await registryService.getProvidersForTenant(req.tenantId);
    res.json({ providers, count: providers.length });
  } catch (err) {
    next(err);
  }
});

// GET /api/v1/registry/providers/:providerId
router.get('/providers/:providerId', async (req, res, next) => {
  try {
    const provider = await registryService.getProvider(req.params.providerId);
    if (!provider) {
      return res.status(404).json({ error: 'Provider not found' });
    }
    res.json(provider);
  } catch (err) {
    next(err);
  }
});

// POST /api/v1/registry/providers
// Add a custom AI provider for this tenant
router.post('/providers', async (req, res, next) => {
  try {
    const { id, name, domains, category, riskTier } = req.body;

    if (!id || !name || !domains || !Array.isArray(domains) || domains.length === 0) {
      return res.status(400).json({
        error: 'id, name, and domains (non-empty array) are required',
      });
    }

    await registryService.addCustomProvider(req.tenantId, {
      id,
      name,
      domains,
      category: category || 'custom',
      riskTier: riskTier || 'medium',
      sniPatterns: req.body.sniPatterns || [],
    });

    res.status(201).json({ message: 'Custom provider added', id });
  } catch (err) {
    next(err);
  }
});

module.exports = router;
