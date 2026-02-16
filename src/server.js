const { createApp } = require('./app');
const config = require('./config');

// Re-export for backward compatibility with existing test imports
const { validateNote } = require('./validators/note.validator');
const {
  createNoteInDb,
  getNotesFromDb,
  getNoteFromDb,
  updateNoteInDb,
  deleteNoteFromDb,
} = require('./services/note.service');

// Start server (only if not in test mode)
if (require.main === module) {
  const app = createApp();

  const server = app.listen(config.port, '0.0.0.0', () => {
    console.log(`Notes API server running on port ${config.port}`);
    console.log(`Region: ${config.region}`);
    console.log(`DynamoDB Table: ${config.tableName}`);
    console.log(`Environment: ${config.nodeEnv}`);
    console.log(`Rate limiting: 100 req/15min global, 20 writes/15min`);
    if (config.dynamoEndpoint) {
      console.log(`DynamoDB Endpoint: ${config.dynamoEndpoint}`);
    }
  });

  // Graceful shutdown
  process.on('SIGTERM', () => {
    console.log('SIGTERM signal received: closing HTTP server');
    server.close(() => {
      console.log('HTTP server closed');
      process.exit(0);
    });
  });
}

// Backward-compatible exports (used by existing tests)
module.exports = {
  createApp,
  validateNote,
  createNoteInDb,
  getNotesFromDb,
  getNoteFromDb,
  updateNoteInDb,
  deleteNoteFromDb,
  config
};
