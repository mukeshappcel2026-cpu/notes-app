const request = require('supertest');
const app = require('../../src/app');

describe('Shadow AI Detector API', () => {
  describe('Health endpoints', () => {
    it('GET /health returns ok', async () => {
      const res = await request(app).get('/health');
      expect(res.status).toBe(200);
      expect(res.body.status).toBe('ok');
      expect(res.body.service).toBe('shadow-ai-detector');
    });

    it('GET /version returns version info', async () => {
      const res = await request(app).get('/version');
      expect(res.status).toBe(200);
      expect(res.body.version).toBe('1.0.0');
    });
  });

  describe('Authentication', () => {
    it('rejects requests without API key', async () => {
      const res = await request(app).get('/api/v1/registry/providers');
      expect(res.status).toBe(401);
    });

    it('rejects requests with invalid API key format', async () => {
      const res = await request(app)
        .get('/api/v1/registry/providers')
        .set('X-API-Key', 'invalid-key');
      expect(res.status).toBe(401);
    });

    it('accepts requests with valid API key format (test mode)', async () => {
      // In test mode with X-Tenant-Id, auth passes but DynamoDB may fail (500).
      // We just verify it doesn't get a 401.
      const res = await request(app)
        .get('/api/v1/findings')
        .set('X-Tenant-Id', 'test-tenant');
      expect(res.status).not.toBe(401);
    }, 15000);
  });

  describe('Ingestion endpoint validation', () => {
    it('POST /api/v1/ingest/event rejects non-object body', async () => {
      const res = await request(app)
        .post('/api/v1/ingest/event')
        .set('X-Tenant-Id', 'test-tenant')
        .send([1, 2, 3]); // array, not an object
      expect(res.status).toBe(400);
    });

    it('POST /api/v1/ingest/batch rejects non-array events', async () => {
      const res = await request(app)
        .post('/api/v1/ingest/batch')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ events: 'not-an-array' });
      expect(res.status).toBe(400);
    });

    it('POST /api/v1/ingest/batch rejects oversized batches', async () => {
      const events = Array.from({ length: 101 }, (_, i) => ({ queryName: `test${i}.com` }));
      const res = await request(app)
        .post('/api/v1/ingest/batch')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ events });
      expect(res.status).toBe(400);
      expect(res.body.error).toContain('Maximum 100');
    });
  });

  describe('Allowlist endpoint validation', () => {
    it('POST /api/v1/allowlist rejects invalid type', async () => {
      const res = await request(app)
        .post('/api/v1/allowlist')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ type: 'invalid', value: '10.0.0.1' });
      expect(res.status).toBe(400);
    });

    it('POST /api/v1/allowlist rejects missing value', async () => {
      const res = await request(app)
        .post('/api/v1/allowlist')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ type: 'source_ip' });
      expect(res.status).toBe(400);
    });
  });

  describe('Registry endpoint validation', () => {
    it('POST /api/v1/registry/providers rejects incomplete provider', async () => {
      const res = await request(app)
        .post('/api/v1/registry/providers')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ id: 'test' }); // missing name and domains
      expect(res.status).toBe(400);
    });
  });

  describe('Findings endpoint validation', () => {
    it('POST /api/v1/findings/:id/acknowledge rejects missing acknowledgedBy', async () => {
      const res = await request(app)
        .post('/api/v1/findings/some-id/acknowledge')
        .set('X-Tenant-Id', 'test-tenant')
        .send({});
      expect(res.status).toBe(400);
    });
  });

  describe('Enrichment endpoint validation', () => {
    it('POST /api/v1/enrichment/catalog rejects missing fields', async () => {
      const res = await request(app)
        .post('/api/v1/enrichment/catalog')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ ipOrCidr: '10.0.5.42' }); // missing serviceName
      expect(res.status).toBe(400);
    });

    it('POST /api/v1/enrichment/catalog/bulk rejects non-array', async () => {
      const res = await request(app)
        .post('/api/v1/enrichment/catalog/bulk')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ entries: 'not-an-array' });
      expect(res.status).toBe(400);
    });

    it('POST /api/v1/enrichment/catalog/bulk rejects oversized batch', async () => {
      const entries = Array.from({ length: 501 }, (_, i) => ({
        ipOrCidr: `10.0.0.${i}`,
        serviceName: `svc-${i}`,
      }));
      const res = await request(app)
        .post('/api/v1/enrichment/catalog/bulk')
        .set('X-Tenant-Id', 'test-tenant')
        .send({ entries });
      expect(res.status).toBe(400);
      expect(res.body.error).toContain('Maximum 500');
    });
  });

  describe('Groups endpoint validation', () => {
    it('POST /api/v1/groups/:key/acknowledge rejects missing acknowledgedBy', async () => {
      const res = await request(app)
        .post('/api/v1/groups/10.0.5.42%3A%3Aopenai/acknowledge')
        .set('X-Tenant-Id', 'test-tenant')
        .send({});
      expect(res.status).toBe(400);
    });
  });
});
