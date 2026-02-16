const express = require('express');
const AWS = require('aws-sdk');
const bodyParser = require('body-parser');
const { v4: uuidv4 } = require('uuid');

// Configuration
const config = {
  port: process.env.PORT || 3000,
  region: process.env.AWS_REGION || 'us-east-1',
  tableName: process.env.DYNAMODB_TABLE || 'Notes',
  nodeEnv: process.env.NODE_ENV || 'development'
};

// Configure AWS SDK
AWS.config.update({ region: config.region });
const dynamodb = new AWS.DynamoDB.DocumentClient();

// Validation functions
function validateNote(data) {
  const { userId, title, content } = data;
  
  if (!userId || typeof userId !== 'string' || userId.trim() === '') {
    return { valid: false, error: 'userId is required and must be a non-empty string' };
  }
  
  if (!title || typeof title !== 'string' || title.trim() === '') {
    return { valid: false, error: 'title is required and must be a non-empty string' };
  }
  
  if (!content || typeof content !== 'string' || content.trim() === '') {
    return { valid: false, error: 'content is required and must be a non-empty string' };
  }
  
  if (title.length > 200) {
    return { valid: false, error: 'title must be 200 characters or less' };
  }
  
  if (content.length > 10000) {
    return { valid: false, error: 'content must be 10000 characters or less' };
  }
  
  return { valid: true };
}

// Database operations
async function createNoteInDb(userId, title, content) {
  const noteId = uuidv4();
  const timestamp = new Date().toISOString();
  
  const params = {
    TableName: config.tableName,
    Item: {
      userId,
      noteId,
      title,
      content,
      createdAt: timestamp,
      updatedAt: timestamp
    }
  };

  await dynamodb.put(params).promise();
  return params.Item;
}

async function getNotesFromDb(userId) {
  const params = {
    TableName: config.tableName,
    KeyConditionExpression: 'userId = :userId',
    ExpressionAttributeValues: {
      ':userId': userId
    }
  };

  const result = await dynamodb.query(params).promise();
  return result.Items || [];
}

async function getNoteFromDb(userId, noteId) {
  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    }
  };

  const result = await dynamodb.get(params).promise();
  return result.Item;
}

async function updateNoteInDb(userId, noteId, title, content) {
  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    },
    UpdateExpression: 'SET title = :title, content = :content, updatedAt = :updatedAt',
    ExpressionAttributeValues: {
      ':title': title,
      ':content': content,
      ':updatedAt': new Date().toISOString()
    },
    ReturnValues: 'ALL_NEW'
  };

  const result = await dynamodb.update(params).promise();
  return result.Attributes;
}

async function deleteNoteFromDb(userId, noteId) {
  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    },
    ReturnValues: 'ALL_OLD'
  };

  const result = await dynamodb.delete(params).promise();
  return result.Attributes;
}

// Create Express app
function createApp() {
  const app = express();

  // Middleware
  app.use(bodyParser.json());

  // CORS middleware
  app.use((req, res, next) => {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
    res.header('Access-Control-Allow-Headers', 'Content-Type, Authorization');
    
    if (req.method === 'OPTIONS') {
      return res.sendStatus(200);
    }
    
    next();
  });

  // Request logging
  if (config.nodeEnv !== 'test') {
    app.use((req, res, next) => {
      console.log(`${new Date().toISOString()} - ${req.method} ${req.path}`);
      next();
    });
  }

  // Routes
  
  // Health check
  app.get('/health', (req, res) => {
    res.json({ 
      status: 'healthy', 
      timestamp: new Date().toISOString(),
      service: 'Notes API',
      version: '1.0.0',
      environment: config.nodeEnv
    });
  });


// Version endpoint (ADD THIS NEW CODE)
  app.get('/version', (req, res) => {
    res.json({
      version: '1.0.0',
      name: 'Notes API',
      description: 'Simple note-taking API',
      author: 'Your Name'
    });
  });


  // Create note
  app.post('/notes', async (req, res) => {
    try {
      const { userId, title, content } = req.body;
      
      // Validation
      const validation = validateNote({ userId, title, content });
      if (!validation.valid) {
        return res.status(400).json({ error: validation.error });
      }
      
      const note = await createNoteInDb(userId, title.trim(), content.trim());
      
      console.log(`Created note ${note.noteId} for user ${userId}`);
      res.status(201).json({ 
        message: 'Note created successfully', 
        note
      });
    } catch (error) {
      console.error('Error creating note:', error);
      res.status(500).json({ 
        error: 'Failed to create note',
        details: config.nodeEnv === 'development' ? error.message : undefined
      });
    }
  });

  // Get all notes for a user
  app.get('/notes/:userId', async (req, res) => {
    try {
      const { userId } = req.params;
      
      if (!userId || userId.trim() === '') {
        return res.status(400).json({ error: 'userId is required' });
      }
      
      const notes = await getNotesFromDb(userId);
      
      console.log(`Retrieved ${notes.length} notes for user ${userId}`);
      res.json({ 
        notes,
        count: notes.length
      });
    } catch (error) {
      console.error('Error getting notes:', error);
      res.status(500).json({ 
        error: 'Failed to retrieve notes',
        details: config.nodeEnv === 'development' ? error.message : undefined
      });
    }
  });

  // Get single note
  app.get('/notes/:userId/:noteId', async (req, res) => {
    try {
      const { userId, noteId } = req.params;
      
      const note = await getNoteFromDb(userId, noteId);
      
      if (!note) {
        return res.status(404).json({ error: 'Note not found' });
      }
      
      res.json({ note });
    } catch (error) {
      console.error('Error getting note:', error);
      res.status(500).json({ 
        error: 'Failed to retrieve note',
        details: config.nodeEnv === 'development' ? error.message : undefined
      });
    }
  });

  // Update note
  app.put('/notes/:userId/:noteId', async (req, res) => {
    try {
      const { userId, noteId } = req.params;
      const { title, content } = req.body;
      
      if (!title || !content) {
        return res.status(400).json({ 
          error: 'Both title and content are required' 
        });
      }
      
      // Check if note exists
      const existingNote = await getNoteFromDb(userId, noteId);
      if (!existingNote) {
        return res.status(404).json({ error: 'Note not found' });
      }
      
      const updatedNote = await updateNoteInDb(userId, noteId, title.trim(), content.trim());
      
      console.log(`Updated note ${noteId} for user ${userId}`);
      res.json({ 
        message: 'Note updated successfully',
        note: updatedNote
      });
    } catch (error) {
      console.error('Error updating note:', error);
      res.status(500).json({ 
        error: 'Failed to update note',
        details: config.nodeEnv === 'development' ? error.message : undefined
      });
    }
  });

  // Delete note
  app.delete('/notes/:userId/:noteId', async (req, res) => {
    try {
      const { userId, noteId } = req.params;
      
      const deletedNote = await deleteNoteFromDb(userId, noteId);
      
      if (!deletedNote) {
        return res.status(404).json({ error: 'Note not found' });
      }
      
      console.log(`Deleted note ${noteId} for user ${userId}`);
      res.json({ 
        message: 'Note deleted successfully',
        noteId
      });
    } catch (error) {
      console.error('Error deleting note:', error);
      res.status(500).json({ 
        error: 'Failed to delete note',
        details: config.nodeEnv === 'development' ? error.message : undefined
      });
    }
  });

  // 404 handler
  app.use((req, res) => {
    res.status(404).json({ 
      error: 'Endpoint not found',
      path: req.path,
      method: req.method
    });
  });

  // Error handler
  app.use((err, req, res, next) => {
    console.error('Unhandled error:', err);
    res.status(500).json({ 
      error: 'Internal server error',
      details: config.nodeEnv === 'development' ? err.message : undefined
    });
  });

  return app;
}

// Start server (only if not in test mode)
if (require.main === module) {
  const app = createApp();
  
  const server = app.listen(config.port, '0.0.0.0', () => {
    console.log(`Notes API server running on port ${config.port}`);
    console.log(`Region: ${config.region}`);
    console.log(`DynamoDB Table: ${config.tableName}`);
    console.log(`Environment: ${config.nodeEnv}`);
    console.log(`CORS enabled for all origins`);
  });

  // Graceful shutdown
  process.on('SIGTERM', () => {
    console.log('SIGTERM signal received: closing HTTP server');
    server.close(() => {
      console.log('HTTP server closed');
      process.exit(0);
    });
  });
}

// Export for testing
module.exports = {
  createApp,
  validateNote,
  createNoteInDb,
  getNotesFromDb,
  getNoteFromDb,
  updateNoteInDb,
  deleteNoteFromDb,
  config
};
