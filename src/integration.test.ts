import { describe, it, expect, vi } from 'vitest';
import { mkdtempSync, writeFileSync, mkdirSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';
import { Scanner } from './scanner.js';
import { Layer1Analyzer } from './analyzers/layer1.js';
import { DeepAnalyzer } from './analyzers/deep.js';
import { CodexStorage } from './storage.js';
import { IndexManifest } from './manifest.js';
import { SkillGenerator } from './skill-generator.js';
import { Pipeline } from './pipeline.js';

describe('Integration: Full Pipeline', () => {
  it('indexes a small project end-to-end', async () => {
    // Create test fixture
    const tmp = mkdtempSync(join(tmpdir(), 'codex-integration-'));
    mkdirSync(join(tmp, 'src'));
    mkdirSync(join(tmp, 'src', 'auth'));

    writeFileSync(join(tmp, 'package.json'), '{"name": "test-app"}');
    writeFileSync(join(tmp, 'src', 'index.ts'), `
import { app } from './app';
app.listen(3000);
`);
    writeFileSync(join(tmp, 'src', 'auth', 'middleware.ts'), `
export function validateToken(req: Request) {
  const token = req.headers.get('authorization');
  return verifyJWT(token);
}

export function verifyJWT(token: string) {
  return token === 'valid';
}
`);

    // Mock LLM
    const mockLlm = {
      complete: vi.fn(async (prompt: string, tier: string) => {
        if (tier === 'haiku') {
          if (prompt.includes('validateToken')) return 'Validates JWT token from Authorization header.';
          if (prompt.includes('verifyJWT')) return 'Verifies JWT token signature.';
          return 'A code unit.';
        }
        // Opus response for deep analysis
        return JSON.stringify({
          relationships: [{
            type: 'calls', from: 'src/auth/middleware.ts::validateToken',
            to: 'src/auth/middleware.ts::verifyJWT', description: 'Delegates verification',
          }],
          domains: [{
            domain: 'Authentication', description: 'Auth system',
            units: ['src/auth/middleware.ts::validateToken', 'src/auth/middleware.ts::verifyJWT'],
            entryPoints: ['src/auth/middleware.ts::validateToken'],
            dataFlow: 'Request -> validate -> verify', concerns: [],
          }],
          system: {
            overview: 'A test web app', architecture: 'Simple',
            domainInteractions: 'Auth guards routes',
            entryPoints: ['src/index.ts'], techStack: ['TypeScript'],
            risks: [],
          },
          patterns: {
            naming: [{ rule: 'camelCase', examples: ['validateToken'], confidence: 'high' }],
            fileOrganization: [], architecture: [], imports: [],
            errorHandling: [], testing: [], domainBoundaries: [],
          },
        });
      }),
    };

    // Mock FAISS
    const storedMemories: any[] = [];
    const mockFaiss = {
      addBatch: vi.fn(async (memories: any[]) => { storedMemories.push(...memories); }),
      deleteBySource: vi.fn(async () => 0),
      health: vi.fn(async () => true),
    };

    const scanner = new Scanner(tmp);
    const layer1 = new Layer1Analyzer(mockLlm as any);
    const deep = new DeepAnalyzer(mockLlm as any);
    const storage = new CodexStorage(mockFaiss as any, 'test-app');
    const manifest = new IndexManifest(tmp, 'test-app');
    const skillGenerator = new SkillGenerator();

    const pipeline = new Pipeline({
      scanner, layer1, deep, storage, manifest, skillGenerator, projectPath: tmp,
    });

    const result = await pipeline.runFull();

    // Verify scan found files
    expect(result.filesProcessed).toBeGreaterThan(0);

    // Verify units were created
    expect(result.unitsGenerated).toBeGreaterThan(0);

    // Verify domains were found
    expect(result.domainsFound).toBe(1);

    // Verify FAISS was populated
    expect(storedMemories.length).toBeGreaterThan(0);

    // Verify different layers are present
    const sources = new Set(storedMemories.map((m: any) => m.source));
    expect(sources.has('codex/test-app/layer:0')).toBe(true);
    expect(sources.has('codex/test-app/layer:1')).toBe(true);

    // Verify manifest was saved
    expect(manifest.isFirstRun()).toBe(false);
  });
});
