const request = require('supertest');
const AWS = require('aws-sdk-mock');
const { createApp } = require('../../src/app');
const { _resetClient } = require('../../src/services/note.service');

describe('API E2E Tests', () => {
  let app;

  beforeAll(() => {
    // Set test environment
    process.env.NODE_ENV = 'test';
    app = createApp();
  });

  beforeEach(() => {
    AWS.restore();
    _resetClient();
  });

  afterAll(() => {
    AWS.restore();
    _resetClient();
  });

  describe('GET /health', () => {
    test('should return health status', async () => {
      const response = await request(app)
        .get('/health')
        .expect(200);

      expect(response.body).toHaveProperty('status', 'healthy');
      expect(response.body).toHaveProperty('timestamp');
      expect(response.body).toHaveProperty('service', 'Notes API');
      expect(response.body).toHaveProperty('version', '1.0.0');
    });
  });

  describe('POST /notes', () => {
    test('should create a note successfully', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
        callback(null, {});
      });

      const response = await request(app)
        .post('/notes')
        .send({
          userId: 'user123',
          title: 'Test Note',
          content: 'Test Content'
        })
        .expect(201);

      expect(response.body.message).toBe('Note created successfully');
      expect(response.body.note).toHaveProperty('userId', 'user123');
      expect(response.body.note).toHaveProperty('title', 'Test Note');
      expect(response.body.note).toHaveProperty('content', 'Test Content');
      expect(response.body.note).toHaveProperty('noteId');
    });

    test('should return 400 for missing userId', async () => {
      const response = await request(app)
        .post('/notes')
        .send({
          title: 'Test Note',
          content: 'Test Content'
        })
        .expect(400);

      expect(response.body.error).toContain('userId');
    });

    test('should return 400 for missing title', async () => {
      const response = await request(app)
        .post('/notes')
        .send({
          userId: 'user123',
          content: 'Test Content'
        })
        .expect(400);

      expect(response.body.error).toContain('title');
    });

    test('should return 400 for missing content', async () => {
      const response = await request(app)
        .post('/notes')
        .send({
          userId: 'user123',
          title: 'Test Note'
        })
        .expect(400);

      expect(response.body.error).toContain('content');
    });

    test('should return 500 on DynamoDB error', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
        callback(new Error('DynamoDB error'), null);
      });

      const response = await request(app)
        .post('/notes')
        .send({
          userId: 'user123',
          title: 'Test Note',
          content: 'Test Content'
        })
        .expect(500);

      expect(response.body.error).toBe('Failed to create note');
    });
  });

  describe('GET /notes/:userId', () => {
    test('should get all notes for a user', async () => {
      const mockNotes = [
        { userId: 'user123', noteId: '1', title: 'Note 1', content: 'Content 1' },
        { userId: 'user123', noteId: '2', title: 'Note 2', content: 'Content 2' }
      ];

      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        callback(null, { Items: mockNotes });
      });

      const response = await request(app)
        .get('/notes/user123')
        .expect(200);

      expect(response.body.notes).toHaveLength(2);
      expect(response.body.count).toBe(2);
      expect(response.body.notes[0].title).toBe('Note 1');
    });

    test('should return empty array when user has no notes', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        callback(null, { Items: [] });
      });

      const response = await request(app)
        .get('/notes/user123')
        .expect(200);

      expect(response.body.notes).toEqual([]);
      expect(response.body.count).toBe(0);
    });

    test('should return 500 on DynamoDB error', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        callback(new Error('Query failed'), null);
      });

      const response = await request(app)
        .get('/notes/user123')
        .expect(500);

      expect(response.body.error).toBe('Failed to retrieve notes');
    });
  });

  describe('GET /notes/:userId/:noteId', () => {
    test('should get a specific note', async () => {
      const mockNote = {
        userId: 'user123',
        noteId: 'note-456',
        title: 'Test Note',
        content: 'Test Content'
      };

      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, { Item: mockNote });
      });

      const response = await request(app)
        .get('/notes/user123/note-456')
        .expect(200);

      expect(response.body.note).toEqual(mockNote);
    });

    test('should return 404 when note not found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {});
      });

      const response = await request(app)
        .get('/notes/user123/nonexistent')
        .expect(404);

      expect(response.body.error).toBe('Note not found');
    });
  });

  describe('PUT /notes/:userId/:noteId', () => {
    test('should update a note successfully', async () => {
      // Mock the get operation (check if exists)
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {
          Item: { userId: 'user123', noteId: 'note-456', title: 'Old', content: 'Old' }
        });
      });

      // Mock the update operation
      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        callback(null, {
          Attributes: {
            userId: 'user123',
            noteId: 'note-456',
            title: 'Updated Title',
            content: 'Updated Content'
          }
        });
      });

      const response = await request(app)
        .put('/notes/user123/note-456')
        .send({
          title: 'Updated Title',
          content: 'Updated Content'
        })
        .expect(200);

      expect(response.body.message).toBe('Note updated successfully');
      expect(response.body.note.title).toBe('Updated Title');
    });

    test('should return 400 for missing title', async () => {
      const response = await request(app)
        .put('/notes/user123/note-456')
        .send({
          content: 'Updated Content'
        })
        .expect(400);

      expect(response.body.error).toContain('title and content are required');
    });

    test('should return 404 when note not found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {});
      });

      const response = await request(app)
        .put('/notes/user123/nonexistent')
        .send({
          title: 'Updated Title',
          content: 'Updated Content'
        })
        .expect(404);

      expect(response.body.error).toBe('Note not found');
    });
  });

  describe('DELETE /notes/:userId/:noteId (soft-delete)', () => {
    test('should soft-delete a note successfully', async () => {
      // deleteNote calls getNote (get) first, then update
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {
          Item: {
            userId: 'user123',
            noteId: 'note-456',
            title: 'Note',
            content: 'Content',
            isDeleted: false,
            deletedAt: null
          }
        });
      });

      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        callback(null, {
          Attributes: {
            userId: 'user123',
            noteId: 'note-456',
            isDeleted: true,
            deletedAt: new Date().toISOString(),
            updatedAt: new Date().toISOString()
          }
        });
      });

      const response = await request(app)
        .delete('/notes/user123/note-456')
        .expect(200);

      expect(response.body.message).toBe('Note deleted successfully');
      expect(response.body.noteId).toBe('note-456');
    });

    test('should return 404 when note not found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {});
      });

      const response = await request(app)
        .delete('/notes/user123/nonexistent')
        .expect(404);

      expect(response.body.error).toBe('Note not found');
    });

    test('should return 404 when note is already soft-deleted', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {
          Item: {
            userId: 'user123',
            noteId: 'note-456',
            isDeleted: true,
            deletedAt: '2024-01-01T00:00:00.000Z'
          }
        });
      });

      const response = await request(app)
        .delete('/notes/user123/note-456')
        .expect(404);

      expect(response.body.error).toBe('Note not found');
    });
  });

  describe('CORS', () => {
    test('should include CORS headers', async () => {
      const response = await request(app)
        .get('/health')
        .expect(200);

      expect(response.headers['access-control-allow-origin']).toBe('*');
      expect(response.headers['access-control-allow-methods']).toContain('GET');
    });

    test('should handle OPTIONS preflight request', async () => {
      const response = await request(app)
        .options('/notes')
        .expect(200);

      expect(response.headers['access-control-allow-origin']).toBe('*');
    });
  });

  describe('404 Handler', () => {
    test('should return 404 for unknown endpoint', async () => {
      const response = await request(app)
        .get('/nonexistent')
        .expect(404);

      expect(response.body.error).toBe('Endpoint not found');
      expect(response.body.path).toBe('/nonexistent');
    });
  });
});
