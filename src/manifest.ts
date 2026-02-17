import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';
import { createHash } from 'crypto';
import type { IndexManifest as ManifestData } from './types.js';

export class IndexManifest {
  private data: ManifestData;
  private manifestPath: string;

  constructor(
    private projectPath: string,
    projectName: string,
  ) {
    this.manifestPath = join(projectPath, '.codex', 'manifest.json');
    this.data = {
      projectPath,
      projectName,
      lastCommit: '',
      lastFullIndex: '',
      fileHashes: {},
      layerTimestamps: {},
      stats: { totalFiles: 0, totalUnits: 0, totalDomains: 0, languages: [] },
    };
  }

  load(): void {
    if (existsSync(this.manifestPath)) {
      const raw = readFileSync(this.manifestPath, 'utf-8');
      this.data = JSON.parse(raw);
    }
  }

  save(): void {
    const dir = join(this.projectPath, '.codex');
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
    writeFileSync(this.manifestPath, JSON.stringify(this.data, null, 2));
  }

  isFirstRun(): boolean {
    return !existsSync(this.manifestPath);
  }

  getLastCommit(): string | null {
    return this.data.lastCommit || null;
  }

  setLastCommit(commit: string): void {
    this.data.lastCommit = commit;
  }

  updateFileHash(filePath: string, hash: string): void {
    this.data.fileHashes[filePath] = hash;
  }

  getFileHash(filePath: string): string | undefined {
    return this.data.fileHashes[filePath];
  }

  updateLayerTimestamp(layer: string): void {
    this.data.layerTimestamps[layer] = new Date().toISOString();
  }

  updateStats(stats: Partial<ManifestData['stats']>): void {
    Object.assign(this.data.stats, stats);
  }

  getChangedFiles(currentHashes: Record<string, string>): {
    added: string[];
    modified: string[];
    removed: string[];
    unchanged: string[];
  } {
    const added: string[] = [];
    const modified: string[] = [];
    const unchanged: string[] = [];
    const removed: string[] = [];

    for (const [path, hash] of Object.entries(currentHashes)) {
      const prev = this.data.fileHashes[path];
      if (!prev) {
        added.push(path);
      } else if (prev !== hash) {
        modified.push(path);
      } else {
        unchanged.push(path);
      }
    }

    for (const path of Object.keys(this.data.fileHashes)) {
      if (!(path in currentHashes)) {
        removed.push(path);
      }
    }

    return { added, modified, removed, unchanged };
  }

  static hashFile(content: string): string {
    return createHash('sha256').update(content).digest('hex').slice(0, 16);
  }
}
