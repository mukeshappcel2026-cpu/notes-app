const app = require('./app');
const config = require('./config');
const { seedRegistry } = require('./services/registry.service');

const server = app.listen(config.port, async () => {
  console.log(`Shadow AI Detector running on port ${config.port} [${config.env}]`);

  if (config.env !== 'test') {
    try {
      await seedRegistry();
      console.log('AI provider registry seeded');
    } catch (err) {
      console.error('Failed to seed registry (non-fatal):', err.message);
    }
  }
});

process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully');
  server.close(() => process.exit(0));
});

module.exports = server;
