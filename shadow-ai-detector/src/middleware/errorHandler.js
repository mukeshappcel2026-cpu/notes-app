const config = require('../config');

function errorHandler(err, req, res, _next) {
  console.error('Unhandled error:', err.message);

  const status = err.statusCode || 500;
  const response = {
    error: err.message || 'Internal server error',
  };

  if (config.env === 'development') {
    response.stack = err.stack;
  }

  res.status(status).json(response);
}

module.exports = { errorHandler };
