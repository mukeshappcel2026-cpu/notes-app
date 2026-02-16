const express = require('express');
const path = require('path');
const bodyParser = require('body-parser');
const { globalLimiter } = require('./middleware/rateLimiter');
const corsMiddleware = require('./middleware/cors');
const requestLogger = require('./middleware/requestLogger');
const healthRoutes = require('./routes/health.routes');
const noteRoutes = require('./routes/note.routes');
const { notFoundHandler, errorHandler } = require('./middleware/errorHandler');

function createApp() {
  const app = express();

  // Middleware
  app.use(bodyParser.json());
  app.use(globalLimiter);
  app.use(corsMiddleware);
  app.use(requestLogger);

  // Serve frontend static files
  app.use(express.static(path.join(__dirname, '..', 'public')));

  // Routes
  app.use(healthRoutes);
  app.use('/notes', noteRoutes);

  // Error handling
  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}

module.exports = { createApp };
