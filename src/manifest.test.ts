import { describe, it, expect } from 'vitest';
import { IndexManifest } from './manifest.js';
import { mkdtempSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

describe('IndexManifest', () => {
  it('creates a new manifest for a fresh project', () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-manifest-'));
    const manifest = new IndexManifest(tmp, 'test-project');

    expect(manifest.isFirstRun()).toBe(true);
    expect(manifest.getLastCommit()).toBeNull();
  });

  it('saves and loads manifest', () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-manifest-'));

    const manifest = new IndexManifest(tmp, 'test-project');
    manifest.updateFileHash('src/index.ts', 'abc123');
    manifest.updateLayerTimestamp('0');
    manifest.save();

    const loaded = new IndexManifest(tmp, 'test-project');
    loaded.load();
    expect(loaded.isFirstRun()).toBe(false);
    expect(loaded.getFileHash('src/index.ts')).toBe('abc123');
  });

  it('detects changed files', () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-manifest-'));

    const manifest = new IndexManifest(tmp, 'test-project');
    manifest.updateFileHash('src/a.ts', 'hash1');
    manifest.updateFileHash('src/b.ts', 'hash2');
    manifest.save();

    const changed = manifest.getChangedFiles({
      'src/a.ts': 'hash1',     // unchanged
      'src/b.ts': 'hash_new',  // changed
      'src/c.ts': 'hash3',     // new
    });

    expect(changed.modified).toContain('src/b.ts');
    expect(changed.added).toContain('src/c.ts');
    expect(changed.unchanged).toContain('src/a.ts');
  });

  it('detects removed files', () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-manifest-'));

    const manifest = new IndexManifest(tmp, 'test-project');
    manifest.updateFileHash('src/old.ts', 'hash_old');
    manifest.save();

    const changed = manifest.getChangedFiles({});
    expect(changed.removed).toContain('src/old.ts');
  });

  it('hashes file content deterministically', () => {
    const hash1 = IndexManifest.hashFile('hello world');
    const hash2 = IndexManifest.hashFile('hello world');
    const hash3 = IndexManifest.hashFile('different content');

    expect(hash1).toBe(hash2);
    expect(hash1).not.toBe(hash3);
    expect(hash1).toHaveLength(16);
  });
});
