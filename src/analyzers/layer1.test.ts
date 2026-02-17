import { describe, it, expect, vi } from 'vitest';
import { Layer1Analyzer } from './layer1.js';
import type { RawChunk } from '../chunker.js';

describe('Layer1Analyzer', () => {
  it('generates unit summaries via Haiku', async () => {
    const mockLlm = {
      complete: vi.fn(async () => 'Validates a JWT token from the Authorization header and returns a boolean.'),
    };

    const analyzer = new Layer1Analyzer(mockLlm as any);

    const chunk: RawChunk = {
      name: 'validateToken',
      kind: 'function',
      lines: [10, 25] as [number, number],
      code: 'export function validateToken(req) { return verify(req.headers.auth); }',
      imports: ['jsonwebtoken'],
      exports: ['validateToken'],
    };

    const unit = await analyzer.analyzeUnit(chunk, 'src/auth/middleware.ts', 'typescript');

    expect(unit.id).toBe('src/auth/middleware.ts::validateToken');
    expect(unit.summary).toContain('JWT');
    expect(unit.layer).toBe(1);
    expect(mockLlm.complete).toHaveBeenCalledTimes(1);
    expect(mockLlm.complete).toHaveBeenCalledWith(
      expect.any(String),
      'haiku',
      expect.any(Object),
    );
  });

  it('builds correct prompt with code and context', async () => {
    let capturedPrompt = '';
    const mockLlm = {
      complete: vi.fn(async (prompt: string) => {
        capturedPrompt = prompt;
        return 'A helper function.';
      }),
    };

    const analyzer = new Layer1Analyzer(mockLlm as any);
    const chunk: RawChunk = {
      name: 'add',
      kind: 'function',
      lines: [1, 3] as [number, number],
      code: 'function add(a, b) { return a + b; }',
      imports: [],
      exports: [],
    };

    await analyzer.analyzeUnit(chunk, 'utils.ts', 'typescript');

    expect(capturedPrompt).toContain('function add');
    expect(capturedPrompt).toContain('utils.ts');
    expect(capturedPrompt).toContain('typescript');
  });

  it('analyzes multiple chunks in a file', async () => {
    const mockLlm = {
      complete: vi.fn(async () => 'A function.'),
    };

    const analyzer = new Layer1Analyzer(mockLlm as any);
    const chunks: RawChunk[] = [
      { name: 'foo', kind: 'function', lines: [1, 3] as [number, number], code: 'function foo() {}', imports: [], exports: [] },
      { name: 'bar', kind: 'function', lines: [5, 7] as [number, number], code: 'function bar() {}', imports: [], exports: [] },
    ];

    const units = await analyzer.analyzeFile(chunks, 'test.ts', 'typescript');

    expect(units).toHaveLength(2);
    expect(units[0].id).toBe('test.ts::foo');
    expect(units[1].id).toBe('test.ts::bar');
    expect(mockLlm.complete).toHaveBeenCalledTimes(2);
  });
});
