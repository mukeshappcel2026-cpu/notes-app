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

module.exports = { validateNote };
