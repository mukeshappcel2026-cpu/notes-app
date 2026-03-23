const express = require('express');
const bodyParser = require('body-parser');
const { errorHandler } = require('./middleware/errorHandler');
const { requestLogger } = require('./middleware/requestLogger');
const { tenantAuth } = require('./middleware/auth');

const { adminAuth } = require('./middleware/adminAuth');
const healthRoutes = require('./routes/health.routes');
const ingestionRoutes = require('./routes/ingestion.routes');
const findingsRoutes = require('./routes/findings.routes');
const registryRoutes = require('./routes/registry.routes');
const allowlistRoutes = require('./routes/allowlist.routes');
const enrichmentRoutes = require('./routes/enrichment.routes');
const groupsRoutes = require('./routes/groups.routes');
const adminRoutes = require('./routes/admin.routes');

const app = express();

app.use(bodyParser.json({ limit: '1mb' }));
app.use(requestLogger);

// Public routes
app.use('/', healthRoutes);

// Tenant-authenticated routes
app.use('/api/v1/ingest', tenantAuth, ingestionRoutes);
app.use('/api/v1/findings', tenantAuth, findingsRoutes);
app.use('/api/v1/registry', tenantAuth, registryRoutes);
app.use('/api/v1/allowlist', tenantAuth, allowlistRoutes);
app.use('/api/v1/enrichment', tenantAuth, enrichmentRoutes);
app.use('/api/v1/groups', tenantAuth, groupsRoutes);

// Admin-authenticated routes
app.use('/api/v1/admin', adminAuth, adminRoutes);

app.use(errorHandler);

module.exports = app;
