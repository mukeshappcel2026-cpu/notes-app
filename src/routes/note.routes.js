const express = require('express');
const config = require('../config');
const { validateNote } = require('../validators/note.validator');
const noteService = require('../services/note.service');
const { writeLimiter } = require('../middleware/rateLimiter');
const { authenticate } = require('../middleware/auth');

const router = express.Router();

// All note routes require authentication
router.use(authenticate);

// Create note (write-limited)
router.post('/', writeLimiter, async (req, res) => {
  try {
    const userId = req.user.userId;
    const { title, content } = req.body;

    const validation = validateNote({ userId, title, content });
    if (!validation.valid) {
      return res.status(400).json({ error: validation.error });
    }

    const note = await noteService.createNote(userId, title.trim(), content.trim());

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

// Get all notes for authenticated user (only non-deleted)
router.get('/', async (req, res) => {
  try {
    const userId = req.user.userId;

    const notes = await noteService.getNotesByUser(userId);

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

// Get single note (returns 404 for soft-deleted notes)
router.get('/:noteId', async (req, res) => {
  try {
    const userId = req.user.userId;
    const { noteId } = req.params;

    const note = await noteService.getNote(userId, noteId);

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

// Update note (write-limited, cannot update soft-deleted notes)
router.put('/:noteId', writeLimiter, async (req, res) => {
  try {
    const userId = req.user.userId;
    const { noteId } = req.params;
    const { title, content } = req.body;

    if (!title || !content) {
      return res.status(400).json({
        error: 'Both title and content are required'
      });
    }

    // Check if note exists and is not deleted
    const existingNote = await noteService.getNote(userId, noteId);
    if (!existingNote) {
      return res.status(404).json({ error: 'Note not found' });
    }

    const updatedNote = await noteService.updateNote(userId, noteId, title.trim(), content.trim());

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

// Soft-delete note (write-limited)
router.delete('/:noteId', writeLimiter, async (req, res) => {
  try {
    const userId = req.user.userId;
    const { noteId } = req.params;

    const deletedNote = await noteService.deleteNote(userId, noteId);

    if (!deletedNote) {
      return res.status(404).json({ error: 'Note not found' });
    }

    console.log(`Soft-deleted note ${noteId} for user ${userId}`);
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

module.exports = router;
