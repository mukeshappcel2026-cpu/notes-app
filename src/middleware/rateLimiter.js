const rateLimit = require('express-rate-limit');

// Global rate limiter: 100 requests per 15-min window per IP
const globalLimiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 100,
  standardHeaders: true,
  legacyHeaders: false,
  message: {
    error: 'Too many requests, please try again later',
    retryAfterMinutes: 15
  }
});

// Write-specific rate limiter: 20 write requests per 15-min window per IP
const writeLimiter = rateLimit({
  windowMs: 15 * 60 * 1000,
  max: 20,
  standardHeaders: true,
  legacyHeaders: false,
  message: {
    error: 'Too many write requests, please try again later',
    retryAfterMinutes: 15
  }
});

module.exports = { globalLimiter, writeLimiter };
