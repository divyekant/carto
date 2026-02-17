import { describe, it, expect, vi } from 'vitest';
import { CodexStorage } from './storage.js';
import type { CodeUnit } from './types.js';

describe('CodexStorage', () => {
  it('stores Layer 0 structure entries', async () => {
    const mockClient = {
      addBatch: vi.fn(async () => {}),
      deleteBySource: vi.fn(async () => 0),
    };

    const storage = new CodexStorage(mockClient as any, 'my-project');
    await storage.storeLayer0([
      {
        path: 'src',
        role: 'source',
        files: [{ path: 'src/index.ts', language: 'typescript', size: 100, lastModified: '', isEntryPoint: true }],
        children: [],
      },
    ]);

    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/my-project/layer:0');
    expect(mockClient.addBatch).toHaveBeenCalledTimes(1);
    const memories = mockClient.addBatch.mock.calls[0][0];
    expect(memories[0].source).toBe('codex/my-project/layer:0');
    expect(memories[0].text).toContain('src');
  });

  it('stores Layer 1 unit entries', async () => {
    const mockClient = {
      addBatch: vi.fn(async () => {}),
      deleteBySource: vi.fn(async () => 0),
    };

    const storage = new CodexStorage(mockClient as any, 'my-project');
    const units: CodeUnit[] = [{
      id: 'src/auth.ts::validate',
      layer: 1, path: 'src/auth.ts', lines: [1, 10] as [number, number],
      language: 'typescript', kind: 'function', name: 'validate',
      summary: 'Validates auth tokens',
      exports: ['validate'], imports: ['jwt'], rawCode: 'function validate() {}',
    }];

    await storage.storeLayer1(units);

    const memories = mockClient.addBatch.mock.calls[0][0];
    expect(memories[0].source).toBe('codex/my-project/layer:1');
    expect(memories[0].text).toContain('validate');
    expect(memories[0].text).toContain('Validates auth tokens');
  });

  it('uses correct source prefix per layer for deep analysis', async () => {
    const mockClient = {
      addBatch: vi.fn(async () => {}),
      deleteBySource: vi.fn(async () => 0),
    };

    const storage = new CodexStorage(mockClient as any, 'test-proj');

    await storage.storeDeepAnalysis({
      relationships: [{ layer: 2, type: 'calls', from: 'a', to: 'b', description: 'calls b' }],
      domains: [{ layer: 3, domain: 'Auth', description: 'Auth domain', units: ['a'], entryPoints: ['a'], dataFlow: 'a->b', concerns: [] }],
      system: { layer: 4, overview: 'A test app', architecture: 'MVC', domainInteractions: '', entryPoints: [], techStack: [], risks: [] },
      patterns: { naming: [], fileOrganization: [], architecture: [], imports: [], errorHandling: [], testing: [], domainBoundaries: [] },
    });

    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:2');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:3');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:4');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/patterns');
  });
});
