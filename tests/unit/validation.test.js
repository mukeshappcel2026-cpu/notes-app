const { validateNote } = require('../../src/validators/note.validator');

describe('validateNote', () => {
  test('should validate a correct note', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'Test Note',
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(true);
    expect(result.error).toBeUndefined();
  });

  test('should reject note without userId', () => {
    const result = validateNote({
      title: 'Test Note',
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('userId');
  });

  test('should reject note with empty userId', () => {
    const result = validateNote({
      userId: '   ',
      title: 'Test Note',
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('userId');
  });

  test('should reject note without title', () => {
    const result = validateNote({
      userId: 'user123',
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('title');
  });

  test('should reject note with empty title', () => {
    const result = validateNote({
      userId: 'user123',
      title: '   ',
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('title');
  });

  test('should reject note without content', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'Test Note'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('content');
  });

  test('should reject note with empty content', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'Test Note',
      content: '   '
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('content');
  });

  test('should reject note with title longer than 200 characters', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'a'.repeat(201),
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('200 characters');
  });

  test('should accept note with title exactly 200 characters', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'a'.repeat(200),
      content: 'This is test content'
    });
    
    expect(result.valid).toBe(true);
  });

  test('should reject note with content longer than 10000 characters', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'Test Note',
      content: 'a'.repeat(10001)
    });
    
    expect(result.valid).toBe(false);
    expect(result.error).toContain('10000 characters');
  });

  test('should accept note with content exactly 10000 characters', () => {
    const result = validateNote({
      userId: 'user123',
      title: 'Test Note',
      content: 'a'.repeat(10000)
    });
    
    expect(result.valid).toBe(true);
  });
});
