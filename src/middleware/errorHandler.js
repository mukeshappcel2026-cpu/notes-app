const config = require('../config');

function notFoundHandler(req, res) {
  res.status(404).json({
    error: 'Endpoint not found',
    path: req.path,
    method: req.method
  });
}

function errorHandler(err, req, res, next) {
  console.error('Unhandled error:', err);
  res.status(500).json({
    error: 'Internal server error',
    details: config.nodeEnv === 'development' ? err.message : undefined
  });
}

module.exports = { notFoundHandler, errorHandler };
