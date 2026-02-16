const express = require('express');
const config = require('../config');

const router = express.Router();

router.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    service: 'Notes API',
    version: '1.0.0',
    environment: config.nodeEnv
  });
});

router.get('/version', (req, res) => {
  res.json({
    version: '1.0.0',
    name: 'Notes API',
    description: 'Simple note-taking API',
    author: 'Your Name'
  });
});

module.exports = router;
