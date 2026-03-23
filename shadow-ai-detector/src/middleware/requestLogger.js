const config = require('../config');

function requestLogger(req, res, next) {
  if (config.env === 'test') return next();

  const start = Date.now();
  res.on('finish', () => {
    console.log(JSON.stringify({
      method: req.method,
      path: req.path,
      status: res.statusCode,
      duration: Date.now() - start,
      tenantId: req.tenantId || 'anonymous',
    }));
  });
  next();
}

module.exports = { requestLogger };
