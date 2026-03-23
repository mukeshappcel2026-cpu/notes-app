const express = require('express');
const router = express.Router();

router.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'shadow-ai-detector', timestamp: new Date().toISOString() });
});

router.get('/version', (req, res) => {
  res.json({ version: '1.0.0', api: 'v1' });
});

module.exports = router;
