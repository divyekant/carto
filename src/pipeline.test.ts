import { describe, it, expect, vi } from 'vitest';
import { mkdtempSync, writeFileSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';
import { Pipeline } from './pipeline.js';

describe('Pipeline', () => {
  it('runs all phases in order for a full index', async () => {
    const callOrder: string[] = [];

    // Create a real temp directory with a real file so readFileSync succeeds
    const tmp = mkdtempSync(join(tmpdir(), 'codex-pipeline-'));
    writeFileSync(join(tmp, 'main.ts'), 'function main() { console.log("hi"); }');

    const mockScanner = {
      scan: vi.fn(async () => {
        callOrder.push('scan');
        return {
          rootPath: tmp,
          directories: [{
            path: '.',
            role: 'source',
            files: [{
              path: 'main.ts',
              language: 'typescript',
              size: 50,
              lastModified: '',
              isEntryPoint: true,
            }],
            children: [],
          }],
          files: [{
            path: 'main.ts',
            language: 'typescript',
            size: 50,
            lastModified: '',
            isEntryPoint: true,
          }],
          manifests: [],
          entryPoints: ['main.ts'],
          languages: ['typescript'],
        };
      }),
    };

    const mockLayer1 = {
      analyzeFile: vi.fn(async () => {
        callOrder.push('layer1');
        return [{
          id: 'main.ts::main',
          layer: 1 as const,
          path: 'main.ts',
          lines: [1, 5] as [number, number],
          language: 'typescript',
          kind: 'function' as const,
          name: 'main',
          summary: 'Entry point',
          exports: [],
          imports: [],
          rawCode: 'function main() { console.log("hi"); }',
        }];
      }),
    };

    const mockDeep = {
      analyze: vi.fn(async () => {
        callOrder.push('deep');
        return {
          relationships: [],
          domains: [],
          system: {
            layer: 4 as const,
            overview: '',
            architecture: '',
            domainInteractions: '',
            entryPoints: [],
            techStack: [],
            risks: [],
          },
          patterns: {
            naming: [],
            fileOrganization: [],
            architecture: [],
            imports: [],
            errorHandling: [],
            testing: [],
            domainBoundaries: [],
          },
        };
      }),
    };

    const mockStorage = {
      storeLayer0: vi.fn(async () => { callOrder.push('store0'); }),
      storeLayer1: vi.fn(async () => { callOrder.push('store1'); }),
      storeDeepAnalysis: vi.fn(async () => { callOrder.push('storeDeep'); }),
    };

    const mockManifest = {
      isFirstRun: () => true,
      load: vi.fn(),
      save: vi.fn(),
      updateFileHash: vi.fn(),
      updateLayerTimestamp: vi.fn(),
      updateStats: vi.fn(),
      setLastCommit: vi.fn(),
    };

    const pipeline = new Pipeline({
      scanner: mockScanner as any,
      layer1: mockLayer1 as any,
      deep: mockDeep as any,
      storage: mockStorage as any,
      manifest: mockManifest as any,
      skillGenerator: null as any,
      projectPath: tmp,
    });

    const result = await pipeline.runFull();

    // Verify phase ordering
    expect(callOrder).toEqual(['scan', 'layer1', 'deep', 'store0', 'store1', 'storeDeep']);

    // Verify result shape
    expect(result.filesProcessed).toBe(1);
    expect(result.unitsGenerated).toBe(1);
    expect(result.domainsFound).toBe(0);
    expect(result.units).toHaveLength(1);
    expect(result.units[0].id).toBe('main.ts::main');

    // Verify manifest was updated
    expect(mockManifest.updateFileHash).toHaveBeenCalledWith('main.ts', expect.any(String));
    expect(mockManifest.updateLayerTimestamp).toHaveBeenCalledTimes(5);
    expect(mockManifest.updateStats).toHaveBeenCalledWith({
      totalFiles: 1,
      totalUnits: 1,
      totalDomains: 0,
      languages: ['typescript'],
    });
    expect(mockManifest.save).toHaveBeenCalled();
  });

  it('skips files with unknown language', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-pipeline-'));
    writeFileSync(join(tmp, 'data.bin'), 'binary stuff');

    const mockScanner = {
      scan: vi.fn(async () => ({
        rootPath: tmp,
        directories: [],
        files: [{ path: 'data.bin', language: 'unknown', size: 12, lastModified: '', isEntryPoint: false }],
        manifests: [],
        entryPoints: [],
        languages: [],
      })),
    };

    const mockLayer1 = { analyzeFile: vi.fn() };
    const mockDeep = {
      analyze: vi.fn(async () => ({
        relationships: [],
        domains: [],
        system: { layer: 4 as const, overview: '', architecture: '', domainInteractions: '', entryPoints: [], techStack: [], risks: [] },
        patterns: { naming: [], fileOrganization: [], architecture: [], imports: [], errorHandling: [], testing: [], domainBoundaries: [] },
      })),
    };

    const mockStorage = {
      storeLayer0: vi.fn(async () => {}),
      storeLayer1: vi.fn(async () => {}),
      storeDeepAnalysis: vi.fn(async () => {}),
    };

    const mockManifest = {
      isFirstRun: () => true,
      load: vi.fn(),
      save: vi.fn(),
      updateFileHash: vi.fn(),
      updateLayerTimestamp: vi.fn(),
      updateStats: vi.fn(),
      setLastCommit: vi.fn(),
    };

    const pipeline = new Pipeline({
      scanner: mockScanner as any,
      layer1: mockLayer1 as any,
      deep: mockDeep as any,
      storage: mockStorage as any,
      manifest: mockManifest as any,
      skillGenerator: null as any,
      projectPath: tmp,
    });

    const result = await pipeline.runFull();

    expect(result.unitsGenerated).toBe(0);
    expect(mockLayer1.analyzeFile).not.toHaveBeenCalled();
  });

  it('handles files that cannot be read gracefully', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-pipeline-'));
    // Don't create the file - it will fail to read

    const mockScanner = {
      scan: vi.fn(async () => ({
        rootPath: tmp,
        directories: [],
        files: [{ path: 'missing.ts', language: 'typescript', size: 100, lastModified: '', isEntryPoint: false }],
        manifests: [],
        entryPoints: [],
        languages: ['typescript'],
      })),
    };

    const mockLayer1 = { analyzeFile: vi.fn() };
    const mockDeep = {
      analyze: vi.fn(async () => ({
        relationships: [],
        domains: [],
        system: { layer: 4 as const, overview: '', architecture: '', domainInteractions: '', entryPoints: [], techStack: [], risks: [] },
        patterns: { naming: [], fileOrganization: [], architecture: [], imports: [], errorHandling: [], testing: [], domainBoundaries: [] },
      })),
    };

    const mockStorage = {
      storeLayer0: vi.fn(async () => {}),
      storeLayer1: vi.fn(async () => {}),
      storeDeepAnalysis: vi.fn(async () => {}),
    };

    const mockManifest = {
      isFirstRun: () => true,
      load: vi.fn(),
      save: vi.fn(),
      updateFileHash: vi.fn(),
      updateLayerTimestamp: vi.fn(),
      updateStats: vi.fn(),
      setLastCommit: vi.fn(),
    };

    const pipeline = new Pipeline({
      scanner: mockScanner as any,
      layer1: mockLayer1 as any,
      deep: mockDeep as any,
      storage: mockStorage as any,
      manifest: mockManifest as any,
      skillGenerator: null as any,
      projectPath: tmp,
    });

    // Should not throw - missing files are caught and skipped
    const result = await pipeline.runFull();

    expect(result.unitsGenerated).toBe(0);
    expect(mockLayer1.analyzeFile).not.toHaveBeenCalled();
  });
});
