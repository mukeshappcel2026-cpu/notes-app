const AWS = require('aws-sdk');
const { v4: uuidv4 } = require('uuid');
const config = require('../config');

// Configure AWS SDK
AWS.config.update({ region: config.region });

// Lazy-init DynamoDB client so aws-sdk-mock can intercept the constructor in tests
let dynamodb;
function getClient() {
  if (!dynamodb) {
    const dynamoOptions = {};
    if (config.dynamoEndpoint) {
      dynamoOptions.endpoint = config.dynamoEndpoint;
    }
    dynamodb = new AWS.DynamoDB.DocumentClient(dynamoOptions);
  }
  return dynamodb;
}

// Creates a note with isDeleted=false and deletedAt=null
async function createNote(userId, title, content) {
  const noteId = uuidv4();
  const timestamp = new Date().toISOString();

  const params = {
    TableName: config.tableName,
    Item: {
      userId,
      noteId,
      title,
      content,
      isDeleted: false,
      deletedAt: null,
      createdAt: timestamp,
      updatedAt: timestamp
    }
  };

  await getClient().put(params).promise();
  return params.Item;
}

// Returns only non-deleted notes for a user
async function getNotesByUser(userId) {
  const params = {
    TableName: config.tableName,
    KeyConditionExpression: 'userId = :userId',
    FilterExpression: 'isDeleted = :isDeleted OR attribute_not_exists(isDeleted)',
    ExpressionAttributeValues: {
      ':userId': userId,
      ':isDeleted': false
    }
  };

  const result = await getClient().query(params).promise();
  return result.Items || [];
}

// Returns a single note; returns null if soft-deleted
async function getNote(userId, noteId) {
  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    }
  };

  const result = await getClient().get(params).promise();
  const item = result.Item;

  // Return null if note is soft-deleted
  if (item && item.isDeleted) {
    return null;
  }

  return item;
}

// Updates title/content; fails if note is soft-deleted
async function updateNote(userId, noteId, title, content) {
  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    },
    UpdateExpression: 'SET title = :title, content = :content, updatedAt = :updatedAt',
    ConditionExpression: 'isDeleted = :notDeleted OR attribute_not_exists(isDeleted)',
    ExpressionAttributeValues: {
      ':title': title,
      ':content': content,
      ':updatedAt': new Date().toISOString(),
      ':notDeleted': false
    },
    ReturnValues: 'ALL_NEW'
  };

  const result = await getClient().update(params).promise();
  return result.Attributes;
}

// Soft-delete: sets isDeleted=true and records deletedAt timestamp
async function deleteNote(userId, noteId) {
  // First check the note exists and isn't already deleted
  const existing = await getNote(userId, noteId);
  if (!existing) {
    return null;
  }

  const params = {
    TableName: config.tableName,
    Key: {
      userId,
      noteId
    },
    UpdateExpression: 'SET isDeleted = :isDeleted, deletedAt = :deletedAt, updatedAt = :updatedAt',
    ExpressionAttributeValues: {
      ':isDeleted': true,
      ':deletedAt': new Date().toISOString(),
      ':updatedAt': new Date().toISOString()
    },
    ReturnValues: 'ALL_NEW'
  };

  const result = await getClient().update(params).promise();
  return result.Attributes;
}

// Reset cached client (used by tests after AWS.restore())
function _resetClient() {
  dynamodb = null;
}

module.exports = {
  // Primary exports (new names)
  createNote,
  getNotesByUser,
  getNote,
  updateNote,
  deleteNote,
  // Backward-compatible aliases (used by existing tests)
  createNoteInDb: createNote,
  getNotesFromDb: getNotesByUser,
  getNoteFromDb: getNote,
  updateNoteInDb: updateNote,
  deleteNoteFromDb: deleteNote,
  // Test helpers
  _resetClient,
};
