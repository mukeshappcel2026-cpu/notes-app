const AWS = require('aws-sdk-mock');
const {
  createNoteInDb,
  getNotesFromDb,
  getNoteFromDb,
  updateNoteInDb,
  deleteNoteFromDb,
  _resetClient
} = require('../../src/services/note.service');

describe('DynamoDB Integration Tests', () => {
  beforeEach(() => {
    // Clear all mocks and reset cached client before each test
    AWS.restore();
    _resetClient();
  });

  afterAll(() => {
    AWS.restore();
    _resetClient();
  });

  describe('createNoteInDb', () => {
    test('should create a note successfully', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
        callback(null, {});
      });

      const note = await createNoteInDb('user123', 'Test Title', 'Test Content');
      
      expect(note).toHaveProperty('userId', 'user123');
      expect(note).toHaveProperty('title', 'Test Title');
      expect(note).toHaveProperty('content', 'Test Content');
      expect(note).toHaveProperty('noteId');
      expect(note).toHaveProperty('createdAt');
      expect(note).toHaveProperty('updatedAt');
    });

    test('should handle DynamoDB errors', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'put', (params, callback) => {
        callback(new Error('DynamoDB error'), null);
      });

      await expect(createNoteInDb('user123', 'Test', 'Content'))
        .rejects.toThrow('DynamoDB error');
    });
  });

  describe('getNotesFromDb', () => {
    test('should retrieve notes for a user', async () => {
      const mockNotes = [
        { userId: 'user123', noteId: '1', title: 'Note 1', content: 'Content 1' },
        { userId: 'user123', noteId: '2', title: 'Note 2', content: 'Content 2' }
      ];

      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        expect(params.KeyConditionExpression).toBe('userId = :userId');
        expect(params.ExpressionAttributeValues[':userId']).toBe('user123');
        callback(null, { Items: mockNotes });
      });

      const notes = await getNotesFromDb('user123');
      
      expect(notes).toHaveLength(2);
      expect(notes[0].title).toBe('Note 1');
      expect(notes[1].title).toBe('Note 2');
    });

    test('should return empty array when no notes found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        callback(null, { Items: [] });
      });

      const notes = await getNotesFromDb('user123');
      expect(notes).toEqual([]);
    });

    test('should handle DynamoDB errors', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'query', (params, callback) => {
        callback(new Error('Query failed'), null);
      });

      await expect(getNotesFromDb('user123'))
        .rejects.toThrow('Query failed');
    });
  });

  describe('getNoteFromDb', () => {
    test('should retrieve a specific note', async () => {
      const mockNote = {
        userId: 'user123',
        noteId: 'note-456',
        title: 'Test Note',
        content: 'Test Content'
      };

      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        expect(params.Key.userId).toBe('user123');
        expect(params.Key.noteId).toBe('note-456');
        callback(null, { Item: mockNote });
      });

      const note = await getNoteFromDb('user123', 'note-456');
      
      expect(note).toEqual(mockNote);
    });

    test('should return undefined when note not found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {});
      });

      const note = await getNoteFromDb('user123', 'nonexistent');
      expect(note).toBeUndefined();
    });
  });

  describe('updateNoteInDb', () => {
    test('should update a note successfully', async () => {
      const updatedNote = {
        userId: 'user123',
        noteId: 'note-456',
        title: 'Updated Title',
        content: 'Updated Content',
        updatedAt: expect.any(String)
      };

      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        expect(params.Key.userId).toBe('user123');
        expect(params.Key.noteId).toBe('note-456');
        expect(params.ExpressionAttributeValues[':title']).toBe('Updated Title');
        expect(params.ExpressionAttributeValues[':content']).toBe('Updated Content');
        callback(null, { Attributes: updatedNote });
      });

      const result = await updateNoteInDb('user123', 'note-456', 'Updated Title', 'Updated Content');
      
      expect(result.title).toBe('Updated Title');
      expect(result.content).toBe('Updated Content');
    });

    test('should handle DynamoDB errors', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        callback(new Error('Update failed'), null);
      });

      await expect(updateNoteInDb('user123', 'note-456', 'Title', 'Content'))
        .rejects.toThrow('Update failed');
    });
  });

  describe('deleteNoteFromDb (soft-delete)', () => {
    test('should soft-delete a note successfully', async () => {
      // deleteNote first calls getNote (get), then does an update
      const existingNote = {
        userId: 'user123',
        noteId: 'note-456',
        title: 'Existing Note',
        content: 'Existing Content',
        isDeleted: false,
        deletedAt: null
      };

      const softDeletedNote = {
        ...existingNote,
        isDeleted: true,
        deletedAt: expect.any(String),
        updatedAt: expect.any(String)
      };

      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, { Item: existingNote });
      });

      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        expect(params.Key.userId).toBe('user123');
        expect(params.Key.noteId).toBe('note-456');
        expect(params.ExpressionAttributeValues[':isDeleted']).toBe(true);
        expect(params.ExpressionAttributeValues[':deletedAt']).toBeDefined();
        callback(null, { Attributes: softDeletedNote });
      });

      const result = await deleteNoteFromDb('user123', 'note-456');

      expect(result).toHaveProperty('isDeleted', true);
      expect(result).toHaveProperty('deletedAt');
    });

    test('should return null when note to delete not found', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {});
      });

      const result = await deleteNoteFromDb('user123', 'nonexistent');
      expect(result).toBeNull();
    });

    test('should return null when note is already soft-deleted', async () => {
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

      const result = await deleteNoteFromDb('user123', 'note-456');
      expect(result).toBeNull();
    });

    test('should handle DynamoDB errors on get', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(new Error('Get failed'), null);
      });

      await expect(deleteNoteFromDb('user123', 'note-456'))
        .rejects.toThrow('Get failed');
    });

    test('should handle DynamoDB errors on update', async () => {
      AWS.mock('DynamoDB.DocumentClient', 'get', (params, callback) => {
        callback(null, {
          Item: {
            userId: 'user123',
            noteId: 'note-456',
            isDeleted: false,
            deletedAt: null
          }
        });
      });

      AWS.mock('DynamoDB.DocumentClient', 'update', (params, callback) => {
        callback(new Error('Update failed'), null);
      });

      await expect(deleteNoteFromDb('user123', 'note-456'))
        .rejects.toThrow('Update failed');
    });
  });
});
