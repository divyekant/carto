import { describe, it, expect } from 'vitest';
import { Chunker } from './chunker.js';

describe('Chunker', () => {
  it('chunks TypeScript functions', () => {
    const code = `
import { foo } from './foo';

export function validateToken(req: Request): boolean {
  const token = req.headers.authorization;
  return verify(token);
}

export function refreshToken(token: string): string {
  return sign({ token }, secret);
}
`;
    const chunks = new Chunker('typescript').chunk(code, 'auth.ts');
    expect(chunks).toHaveLength(2);
    expect(chunks[0].name).toBe('validateToken');
    expect(chunks[0].kind).toBe('function');
    expect(chunks[1].name).toBe('refreshToken');
  });

  it('chunks Python functions and classes', () => {
    const code = `
import os

class UserService:
    def __init__(self, db):
        self.db = db

    def get_user(self, user_id):
        return self.db.find(user_id)

def helper_function():
    return 42
`;
    const chunks = new Chunker('python').chunk(code, 'service.py');
    expect(chunks).toHaveLength(2);
    expect(chunks[0].name).toBe('UserService');
    expect(chunks[0].kind).toBe('class');
    expect(chunks[1].name).toBe('helper_function');
    expect(chunks[1].kind).toBe('function');
  });

  it('treats config files as a single chunk', () => {
    const code = '{"name": "my-app", "version": "1.0.0"}';
    const chunks = new Chunker('json').chunk(code, 'package.json');
    expect(chunks).toHaveLength(1);
    expect(chunks[0].kind).toBe('config');
    expect(chunks[0].name).toBe('package.json');
  });

  it('treats script files with no declarations as a single chunk', () => {
    const code = `
echo "hello"
apt-get update
npm install
`;
    const chunks = new Chunker('shell').chunk(code, 'setup.sh');
    expect(chunks).toHaveLength(1);
    expect(chunks[0].kind).toBe('script');
  });

  it('extracts imports', () => {
    const code = `
import { Router } from 'express';
import { validate } from '../utils/validate';

export function createRouter() {
  return Router();
}
`;
    const chunks = new Chunker('typescript').chunk(code, 'router.ts');
    expect(chunks[0].imports).toContain('express');
    expect(chunks[0].imports).toContain('../utils/validate');
  });
});
