const config = require('../config');

function requestLogger(req, res, next) {
  if (config.nodeEnv === 'test') {
    return next();
  }

  const start = Date.now();
  res.on('finish', () => {
    console.log(JSON.stringify({
      timestamp: new Date().toISOString(),
      method: req.method,
      path: req.path,
      statusCode: res.statusCode,
      durationMs: Date.now() - start,
      ip: req.ip
    }));
  });
  next();
}

module.exports = requestLogger;
