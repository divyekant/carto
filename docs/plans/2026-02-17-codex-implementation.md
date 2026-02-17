# Codex Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a CLI tool that indexes any codebase into a layered context graph stored in FAISS, queryable by AI agents and humans.

**Architecture:** CLI-first TypeScript app. Scanner walks the file tree and chunks files into units. LLM analyzers (Haiku for bulk, Opus for deep analysis) enrich chunks with summaries, relationships, domains, and system narrative. All context stored in FAISS via the existing REST API at localhost:8900. A pattern enforcement module generates CLAUDE.md/.cursorrules from discovered conventions.

**Tech Stack:** TypeScript, Node.js, Anthropic Claude SDK (@anthropic-ai/sdk), FAISS REST API (localhost:8900), Commander.js (CLI), Vitest (testing)

**FAISS API Reference:**
- Base URL: `http://localhost:8900`, Header: `X-API-Key: god-is-an-astronaut`
- `POST /memory/add` - `{text, source, metadata?, deduplicate}` → `{success, id}`
- `POST /memory/add-batch` - `{memories: [{text, source, metadata?}], deduplicate}` (max 500)
- `POST /search` - `{query, k, threshold?, hybrid}` → `{results[]}`
- `GET /memories` - `?offset=N&limit=N&source=filter` → paginated list
- `DELETE /memory/{id}` - Delete by ID
- `POST /memory/is-novel` - `{text, threshold?}` → `{is_novel, most_similar?}`

---

## Task 1: Project Scaffolding

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `vitest.config.ts`
- Create: `src/index.ts` (CLI entry point)
- Create: `src/types.ts` (shared type definitions)
- Create: `.gitignore`

**Step 1: Initialize the project**

```bash
cd /Users/dk/projects/indexer
npm init -y
```

**Step 2: Install dependencies**

```bash
npm install typescript @anthropic-ai/sdk commander chalk ora glob ignore
npm install -D vitest @types/node tsx
```

- `commander` - CLI framework
- `chalk` - Terminal colors
- `ora` - Spinners for progress
- `glob` - File pattern matching
- `ignore` - .gitignore parsing
- `@anthropic-ai/sdk` - Claude API
- `tsx` - TypeScript execution

**Step 3: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "declaration": true,
    "resolveJsonModule": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "**/*.test.ts"]
}
```

**Step 4: Create vitest.config.ts**

```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
  },
});
```

**Step 5: Create .gitignore**

```
node_modules/
dist/
.codex/
*.tgz
```

**Step 6: Create src/types.ts with core type definitions**

```typescript
// Layer 0 - Structure
export interface DirectoryNode {
  path: string;
  role: string; // 'source' | 'test' | 'config' | 'docs' | 'build' | 'unknown'
  files: FileInfo[];
  children: string[]; // child directory paths
}

export interface FileInfo {
  path: string;
  language: string;
  size: number;
  lastModified: string;
  isEntryPoint: boolean;
}

// Layer 1 - Units
export interface CodeUnit {
  id: string; // "path/file.ts::functionName"
  layer: 1;
  path: string;
  lines: [number, number];
  language: string;
  kind: 'function' | 'class' | 'interface' | 'type' | 'module' | 'config' | 'script';
  name: string;
  summary: string;
  exports: string[];
  imports: string[];
  rawCode: string;
}

// Layer 2 - Relationships
export interface Relationship {
  layer: 2;
  type: 'calls' | 'imports' | 'implements' | 'extends' | 'configures' | 'uses';
  from: string; // unit id
  to: string; // unit id
  description: string;
}

// Layer 3 - Domains
export interface Domain {
  layer: 3;
  domain: string;
  description: string;
  units: string[]; // unit ids
  entryPoints: string[];
  dataFlow: string;
  concerns: string[];
}

// Layer 4 - System
export interface SystemNarrative {
  layer: 4;
  overview: string;
  architecture: string;
  domainInteractions: string;
  entryPoints: string[];
  techStack: string[];
  risks: string[];
}

// Pattern Enforcement
export interface CodebasePatterns {
  naming: PatternRule[];
  fileOrganization: PatternRule[];
  architecture: PatternRule[];
  imports: PatternRule[];
  errorHandling: PatternRule[];
  testing: PatternRule[];
  domainBoundaries: PatternRule[];
}

export interface PatternRule {
  rule: string;
  examples: string[];
  confidence: 'high' | 'medium' | 'low';
}

// Index Manifest
export interface IndexManifest {
  projectPath: string;
  projectName: string;
  lastCommit: string;
  lastFullIndex: string;
  fileHashes: Record<string, string>;
  layerTimestamps: Record<string, string>;
  stats: {
    totalFiles: number;
    totalUnits: number;
    totalDomains: number;
    languages: string[];
  };
}

// FAISS storage wrapper
export interface FaissMemory {
  text: string;
  source: string;
  metadata?: Record<string, unknown>;
}

// Config
export interface CodexConfig {
  faissUrl: string;
  faissApiKey: string;
  anthropicApiKey: string;
  haikuModel: string;
  opusModel: string;
  maxConcurrentLlmCalls: number;
}
```

**Step 7: Create src/index.ts with CLI skeleton**

```typescript
#!/usr/bin/env node
import { Command } from 'commander';

const program = new Command();

program
  .name('codex')
  .description('Codebase intelligence - index any codebase into a layered context graph')
  .version('0.1.0');

program
  .command('index')
  .description('Index a codebase')
  .argument('<path>', 'Path to the codebase to index')
  .option('--full', 'Force full re-index (ignore cache)')
  .option('--layers <layers>', 'Comma-separated layers to run (0,1,2,3,4)', '0,1,2,3,4')
  .option('--dry-run', 'Show what would be indexed without making LLM calls')
  .action(async (path: string, options) => {
    console.log(`Indexing ${path}...`);
    // Will be implemented in later tasks
  });

program
  .command('query')
  .description('Query the indexed codebase')
  .argument('<query>', 'Natural language query')
  .option('--layer <layer>', 'Filter by layer (0-4)')
  .option('--domain <domain>', 'Filter by domain name')
  .option('-k <count>', 'Number of results', '10')
  .action(async (query: string, options) => {
    console.log(`Querying: ${query}`);
    // Will be implemented in later tasks
  });

program
  .command('patterns')
  .description('Generate pattern enforcement files (CLAUDE.md, .cursorrules)')
  .argument('<path>', 'Path to the codebase')
  .option('--format <format>', 'Output format: claude, cursor, all', 'all')
  .action(async (path: string, options) => {
    console.log(`Generating patterns for ${path}...`);
    // Will be implemented in later tasks
  });

program
  .command('status')
  .description('Show index status for a codebase')
  .argument('<path>', 'Path to the codebase')
  .action(async (path: string) => {
    console.log(`Status for ${path}...`);
    // Will be implemented in later tasks
  });

program.parse();
```

**Step 8: Add scripts to package.json**

Update `package.json` to add:
```json
{
  "type": "module",
  "bin": {
    "codex": "./dist/index.js"
  },
  "scripts": {
    "dev": "tsx src/index.ts",
    "build": "tsc",
    "test": "vitest run",
    "test:watch": "vitest"
  }
}
```

**Step 9: Verify setup**

```bash
npx tsx src/index.ts --help
npx vitest run
```

Expected: CLI help output, test runner reports 0 tests.

**Step 10: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding - CLI skeleton, types, tooling"
```

---

## Task 2: Configuration & FAISS Client

**Files:**
- Create: `src/config.ts`
- Create: `src/faiss-client.ts`
- Create: `src/faiss-client.test.ts`

**Step 1: Write failing test for config loading**

```typescript
// src/faiss-client.test.ts
import { describe, it, expect } from 'vitest';
import { loadConfig } from './config.js';

describe('loadConfig', () => {
  it('loads config from environment variables', () => {
    const config = loadConfig();
    expect(config.faissUrl).toBeDefined();
    expect(config.anthropicApiKey).toBeDefined();
    expect(config.haikuModel).toBeDefined();
    expect(config.opusModel).toBeDefined();
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/faiss-client.test.ts
```

Expected: FAIL - module not found.

**Step 3: Implement config.ts**

```typescript
// src/config.ts
import type { CodexConfig } from './types.js';

export function loadConfig(): CodexConfig {
  return {
    faissUrl: process.env.FAISS_URL || 'http://localhost:8900',
    faissApiKey: process.env.FAISS_API_KEY || 'god-is-an-astronaut',
    anthropicApiKey: process.env.ANTHROPIC_API_KEY || '',
    haikuModel: process.env.CODEX_HAIKU_MODEL || 'claude-haiku-4-5-20251001',
    opusModel: process.env.CODEX_OPUS_MODEL || 'claude-opus-4-6',
    maxConcurrentLlmCalls: parseInt(process.env.CODEX_MAX_CONCURRENT || '5', 10),
  };
}
```

**Step 4: Write failing tests for FAISS client**

Add to `src/faiss-client.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { FaissClient } from './faiss-client.js';

// Mock fetch globally
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

describe('FaissClient', () => {
  let client: FaissClient;

  beforeEach(() => {
    client = new FaissClient('http://localhost:8900', 'test-key');
    mockFetch.mockReset();
  });

  it('adds a memory with correct headers and body', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, id: 42 }),
    });

    const id = await client.addMemory({
      text: 'test memory',
      source: 'test/source',
      metadata: { layer: 1 },
    });

    expect(id).toBe(42);
    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8900/memory/add',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'X-API-Key': 'test-key',
        }),
      }),
    );
  });

  it('searches with hybrid mode', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        results: [{ id: 1, text: 'result', score: 0.9, source: 'test' }],
        count: 1,
      }),
    });

    const results = await client.search('auth token', { k: 5 });
    expect(results).toHaveLength(1);
    expect(results[0].score).toBe(0.9);
  });

  it('batch adds memories', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, added: 3 }),
    });

    await client.addBatch([
      { text: 'mem1', source: 's1' },
      { text: 'mem2', source: 's2' },
      { text: 'mem3', source: 's3' },
    ]);

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.memories).toHaveLength(3);
  });

  it('deletes memories by source prefix', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        memories: [
          { id: 1, text: 't1', source: 'codex/proj/layer:0' },
          { id: 2, text: 't2', source: 'codex/proj/layer:0' },
        ],
        total: 2,
      }),
    });
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({ success: true }) });
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({ success: true }) });

    const count = await client.deleteBySource('codex/proj/layer:0');
    expect(count).toBe(2);
  });
});
```

**Step 5: Run tests to verify they fail**

```bash
npx vitest run src/faiss-client.test.ts
```

**Step 6: Implement faiss-client.ts**

```typescript
// src/faiss-client.ts

export interface SearchResult {
  id: number;
  text: string;
  score: number;
  source: string;
  metadata?: Record<string, unknown>;
}

export interface SearchOptions {
  k?: number;
  threshold?: number;
  hybrid?: boolean;
  source?: string;
}

export class FaissClient {
  constructor(
    private baseUrl: string,
    private apiKey: string,
  ) {}

  private async request(path: string, options: RequestInit = {}): Promise<Response> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': this.apiKey,
        ...options.headers,
      },
    });
    if (!res.ok) {
      const text = await res.text().catch(() => 'unknown error');
      throw new Error(`FAISS API error ${res.status}: ${text}`);
    }
    return res;
  }

  async addMemory(memory: {
    text: string;
    source: string;
    metadata?: Record<string, unknown>;
    deduplicate?: boolean;
  }): Promise<number> {
    const res = await this.request('/memory/add', {
      method: 'POST',
      body: JSON.stringify({
        text: memory.text,
        source: memory.source,
        metadata: memory.metadata,
        deduplicate: memory.deduplicate ?? false,
      }),
    });
    const data = await res.json();
    return data.id;
  }

  async addBatch(
    memories: { text: string; source: string; metadata?: Record<string, unknown> }[],
    deduplicate = false,
  ): Promise<void> {
    // FAISS API allows max 500 per batch
    for (let i = 0; i < memories.length; i += 500) {
      const batch = memories.slice(i, i + 500);
      await this.request('/memory/add-batch', {
        method: 'POST',
        body: JSON.stringify({ memories: batch, deduplicate }),
      });
    }
  }

  async search(query: string, options: SearchOptions = {}): Promise<SearchResult[]> {
    const res = await this.request('/search', {
      method: 'POST',
      body: JSON.stringify({
        query,
        k: options.k ?? 10,
        threshold: options.threshold,
        hybrid: options.hybrid ?? true,
      }),
    });
    const data = await res.json();
    return data.results;
  }

  async listBySource(source: string, limit = 50): Promise<SearchResult[]> {
    const res = await this.request(`/memories?source=${encodeURIComponent(source)}&limit=${limit}`);
    const data = await res.json();
    return data.memories;
  }

  async deleteMemory(id: number): Promise<void> {
    await this.request(`/memory/${id}`, { method: 'DELETE' });
  }

  async deleteBySource(sourcePrefix: string): Promise<number> {
    const memories = await this.listBySource(sourcePrefix);
    for (const mem of memories) {
      await this.deleteMemory(mem.id);
    }
    return memories.length;
  }

  async health(): Promise<boolean> {
    try {
      const res = await this.request('/health');
      return res.ok;
    } catch {
      return false;
    }
  }
}
```

**Step 7: Run tests to verify they pass**

```bash
npx vitest run src/faiss-client.test.ts
```

Expected: All 4 tests PASS.

**Step 8: Commit**

```bash
git add -A
git commit -m "feat: config loader and FAISS REST client with tests"
```

---

## Task 3: Layer 0 - File Scanner

**Files:**
- Create: `src/scanner.ts`
- Create: `src/scanner.test.ts`
- Create: `src/languages.ts`

**Step 1: Write failing tests for the scanner**

```typescript
// src/scanner.test.ts
import { describe, it, expect } from 'vitest';
import { detectLanguage, inferDirectoryRole, isEntryPoint } from './languages.js';

describe('detectLanguage', () => {
  it('detects TypeScript', () => expect(detectLanguage('foo.ts')).toBe('typescript'));
  it('detects Python', () => expect(detectLanguage('app.py')).toBe('python'));
  it('detects Go', () => expect(detectLanguage('main.go')).toBe('go'));
  it('detects Rust', () => expect(detectLanguage('lib.rs')).toBe('rust'));
  it('returns unknown for unrecognized', () => expect(detectLanguage('file.xyz')).toBe('unknown'));
  it('detects JSON', () => expect(detectLanguage('config.json')).toBe('json'));
  it('detects YAML', () => expect(detectLanguage('docker-compose.yml')).toBe('yaml'));
});

describe('inferDirectoryRole', () => {
  it('identifies source dirs', () => expect(inferDirectoryRole('src')).toBe('source'));
  it('identifies test dirs', () => expect(inferDirectoryRole('tests')).toBe('test'));
  it('identifies test dirs variant', () => expect(inferDirectoryRole('__tests__')).toBe('test'));
  it('identifies config dirs', () => expect(inferDirectoryRole('config')).toBe('config'));
  it('identifies docs dirs', () => expect(inferDirectoryRole('docs')).toBe('docs'));
  it('defaults to unknown', () => expect(inferDirectoryRole('foo')).toBe('unknown'));
});

describe('isEntryPoint', () => {
  it('identifies main files', () => expect(isEntryPoint('main.ts')).toBe(true));
  it('identifies index files', () => expect(isEntryPoint('index.js')).toBe(true));
  it('identifies app files', () => expect(isEntryPoint('app.py')).toBe(true));
  it('rejects random files', () => expect(isEntryPoint('utils.ts')).toBe(false));
});
```

**Step 2: Run tests to verify they fail**

```bash
npx vitest run src/scanner.test.ts
```

**Step 3: Implement languages.ts**

```typescript
// src/languages.ts

const EXTENSION_MAP: Record<string, string> = {
  '.ts': 'typescript', '.tsx': 'typescript',
  '.js': 'javascript', '.jsx': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript',
  '.py': 'python', '.pyw': 'python',
  '.go': 'go',
  '.rs': 'rust',
  '.java': 'java',
  '.rb': 'ruby',
  '.php': 'php',
  '.cs': 'csharp',
  '.cpp': 'cpp', '.cc': 'cpp', '.cxx': 'cpp', '.hpp': 'cpp', '.h': 'cpp',
  '.c': 'c',
  '.swift': 'swift',
  '.kt': 'kotlin', '.kts': 'kotlin',
  '.scala': 'scala',
  '.ex': 'elixir', '.exs': 'elixir',
  '.erl': 'erlang',
  '.hs': 'haskell',
  '.lua': 'lua',
  '.r': 'r', '.R': 'r',
  '.dart': 'dart',
  '.vue': 'vue',
  '.svelte': 'svelte',
  '.sql': 'sql',
  '.sh': 'shell', '.bash': 'shell', '.zsh': 'shell',
  '.json': 'json',
  '.yaml': 'yaml', '.yml': 'yaml',
  '.toml': 'toml',
  '.xml': 'xml',
  '.html': 'html', '.htm': 'html',
  '.css': 'css', '.scss': 'scss', '.less': 'less',
  '.md': 'markdown', '.mdx': 'markdown',
  '.dockerfile': 'dockerfile',
  '.tf': 'terraform',
  '.proto': 'protobuf',
  '.graphql': 'graphql', '.gql': 'graphql',
};

const DIRECTORY_ROLES: Record<string, string> = {
  'src': 'source', 'lib': 'source', 'app': 'source', 'pkg': 'source', 'packages': 'source',
  'test': 'test', 'tests': 'test', '__tests__': 'test', 'spec': 'test', 'specs': 'test',
  'e2e': 'test', 'integration': 'test',
  'config': 'config', 'configs': 'config', 'configuration': 'config',
  'docs': 'docs', 'doc': 'docs', 'documentation': 'docs',
  'build': 'build', 'dist': 'build', 'out': 'build', 'target': 'build',
  'scripts': 'scripts', 'bin': 'scripts', 'tools': 'scripts',
  'public': 'static', 'static': 'static', 'assets': 'static',
  'vendor': 'vendor', 'node_modules': 'vendor', 'third_party': 'vendor',
  'migrations': 'migrations',
};

const ENTRY_POINT_PATTERNS = [
  /^main\.\w+$/,
  /^index\.\w+$/,
  /^app\.\w+$/,
  /^server\.\w+$/,
  /^cli\.\w+$/,
  /^mod\.\w+$/,
  /^__main__\.py$/,
  /^manage\.py$/,
];

const MANIFEST_FILES = new Set([
  'package.json', 'requirements.txt', 'Pipfile', 'pyproject.toml',
  'go.mod', 'Cargo.toml', 'pom.xml', 'build.gradle', 'build.gradle.kts',
  'Gemfile', 'composer.json', 'mix.exs', 'Package.swift',
  'pubspec.yaml', 'CMakeLists.txt', 'Makefile',
]);

import { extname, basename } from 'path';

export function detectLanguage(filePath: string): string {
  const ext = extname(filePath).toLowerCase();
  if (EXTENSION_MAP[ext]) return EXTENSION_MAP[ext];

  const name = basename(filePath).toLowerCase();
  if (name === 'dockerfile' || name.startsWith('dockerfile.')) return 'dockerfile';
  if (name === 'makefile') return 'makefile';

  return 'unknown';
}

export function inferDirectoryRole(dirName: string): string {
  return DIRECTORY_ROLES[dirName.toLowerCase()] || 'unknown';
}

export function isEntryPoint(filePath: string): boolean {
  const name = basename(filePath);
  return ENTRY_POINT_PATTERNS.some(p => p.test(name));
}

export function isManifestFile(filePath: string): boolean {
  return MANIFEST_FILES.has(basename(filePath));
}

export function isCodeFile(filePath: string): boolean {
  const lang = detectLanguage(filePath);
  return lang !== 'unknown' && !['json', 'yaml', 'toml', 'xml', 'markdown'].includes(lang);
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/scanner.test.ts
```

**Step 5: Write failing test for the scanner itself**

Add to `src/scanner.test.ts`:

```typescript
import { Scanner } from './scanner.js';
import { mkdtempSync, writeFileSync, mkdirSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

describe('Scanner', () => {
  it('scans a directory and returns structure', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-test-'));
    mkdirSync(join(tmp, 'src'));
    writeFileSync(join(tmp, 'src', 'index.ts'), 'export const x = 1;');
    writeFileSync(join(tmp, 'src', 'utils.ts'), 'export function add(a: number, b: number) { return a + b; }');
    writeFileSync(join(tmp, 'package.json'), '{"name": "test"}');
    mkdirSync(join(tmp, 'tests'));
    writeFileSync(join(tmp, 'tests', 'utils.test.ts'), 'test("add", () => {});');

    const scanner = new Scanner(tmp);
    const result = await scanner.scan();

    expect(result.directories.length).toBeGreaterThan(0);
    expect(result.files.length).toBe(4);
    expect(result.manifests).toContain('package.json');
    expect(result.entryPoints).toContain('src/index.ts');
    expect(result.languages).toContain('typescript');
  });

  it('respects .gitignore', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'codex-test-'));
    writeFileSync(join(tmp, '.gitignore'), 'ignored/\n*.log');
    mkdirSync(join(tmp, 'ignored'));
    writeFileSync(join(tmp, 'ignored', 'secret.ts'), 'const secret = "x";');
    writeFileSync(join(tmp, 'app.log'), 'log line');
    writeFileSync(join(tmp, 'main.ts'), 'console.log("hi");');

    const scanner = new Scanner(tmp);
    const result = await scanner.scan();

    const paths = result.files.map(f => f.path);
    expect(paths).not.toContain('ignored/secret.ts');
    expect(paths).not.toContain('app.log');
    expect(paths).toContain('main.ts');
  });
});
```

**Step 6: Run test to verify it fails**

```bash
npx vitest run src/scanner.test.ts
```

**Step 7: Implement scanner.ts**

```typescript
// src/scanner.ts
import { readdirSync, statSync, readFileSync, existsSync } from 'fs';
import { join, relative, basename, dirname } from 'path';
import ignore from 'ignore';
import { detectLanguage, inferDirectoryRole, isEntryPoint, isManifestFile } from './languages.js';
import type { DirectoryNode, FileInfo } from './types.js';

export interface ScanResult {
  rootPath: string;
  directories: DirectoryNode[];
  files: FileInfo[];
  manifests: string[];
  entryPoints: string[];
  languages: string[];
}

const DEFAULT_IGNORE = [
  'node_modules', '.git', '.svn', '.hg', 'dist', 'build', 'out',
  '__pycache__', '.pytest_cache', '.mypy_cache', '.tox',
  'target', '.gradle', '.idea', '.vscode', '.DS_Store',
  'vendor', 'coverage', '.next', '.nuxt', '.turbo',
];

export class Scanner {
  private ig: ReturnType<typeof ignore>;

  constructor(private rootPath: string) {
    this.ig = ignore();
    this.ig.add(DEFAULT_IGNORE);

    const gitignorePath = join(rootPath, '.gitignore');
    if (existsSync(gitignorePath)) {
      const content = readFileSync(gitignorePath, 'utf-8');
      this.ig.add(content);
    }
  }

  async scan(): Promise<ScanResult> {
    const directories: DirectoryNode[] = [];
    const files: FileInfo[] = [];
    const manifests: string[] = [];
    const entryPoints: string[] = [];
    const languageSet = new Set<string>();

    this.walkDir(this.rootPath, '', directories, files, manifests, entryPoints, languageSet);

    return {
      rootPath: this.rootPath,
      directories,
      files,
      manifests,
      entryPoints,
      languages: [...languageSet],
    };
  }

  private walkDir(
    absPath: string,
    relPath: string,
    directories: DirectoryNode[],
    files: FileInfo[],
    manifests: string[],
    entryPoints: string[],
    languageSet: Set<string>,
  ): void {
    const entries = readdirSync(absPath, { withFileTypes: true });
    const dirFiles: FileInfo[] = [];
    const childDirs: string[] = [];

    for (const entry of entries) {
      const entryRelPath = relPath ? `${relPath}/${entry.name}` : entry.name;

      if (this.ig.ignores(entryRelPath)) continue;

      if (entry.isDirectory()) {
        childDirs.push(entryRelPath);
        this.walkDir(
          join(absPath, entry.name), entryRelPath,
          directories, files, manifests, entryPoints, languageSet,
        );
      } else if (entry.isFile()) {
        const stat = statSync(join(absPath, entry.name));
        const language = detectLanguage(entry.name);

        const fileInfo: FileInfo = {
          path: entryRelPath,
          language,
          size: stat.size,
          lastModified: stat.mtime.toISOString(),
          isEntryPoint: isEntryPoint(entry.name),
        };

        dirFiles.push(fileInfo);
        files.push(fileInfo);

        if (language !== 'unknown') languageSet.add(language);
        if (isManifestFile(entry.name)) manifests.push(entryRelPath);
        if (fileInfo.isEntryPoint) entryPoints.push(entryRelPath);
      }
    }

    directories.push({
      path: relPath || '.',
      role: inferDirectoryRole(basename(absPath)),
      files: dirFiles,
      children: childDirs,
    });
  }
}
```

**Step 8: Run tests to verify they pass**

```bash
npx vitest run src/scanner.test.ts
```

Expected: All tests PASS.

**Step 9: Commit**

```bash
git add -A
git commit -m "feat: Layer 0 - file scanner with language detection and gitignore support"
```

---

## Task 4: Code Chunker (Language-Agnostic)

**Files:**
- Create: `src/chunker.ts`
- Create: `src/chunker.test.ts`

The chunker splits source files into logical units. It uses heuristic pattern matching (not AST) to stay language-agnostic. It detects top-level declarations by looking for common patterns across languages.

**Step 1: Write failing tests**

```typescript
// src/chunker.test.ts
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
```

**Step 2: Run tests to verify they fail**

```bash
npx vitest run src/chunker.test.ts
```

**Step 3: Implement chunker.ts**

The chunker uses regex-based heuristics per language family. It groups languages into families (c-like, python, etc.) and applies the appropriate splitting strategy.

```typescript
// src/chunker.ts
import { basename } from 'path';

export interface RawChunk {
  name: string;
  kind: 'function' | 'class' | 'interface' | 'type' | 'module' | 'config' | 'script';
  lines: [number, number]; // 1-indexed
  code: string;
  imports: string[];
  exports: string[];
}

const CONFIG_LANGUAGES = new Set(['json', 'yaml', 'toml', 'xml']);
const SCRIPT_LANGUAGES = new Set(['shell', 'dockerfile', 'makefile']);

// Patterns for top-level declaration detection per language family
const DECLARATION_PATTERNS: Record<string, RegExp[]> = {
  clike: [
    /^(?:export\s+)?(?:async\s+)?function\s+(\w+)/,
    /^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)/,
    /^(?:export\s+)?interface\s+(\w+)/,
    /^(?:export\s+)?type\s+(\w+)\s*=/,
    /^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(/,
    /^(?:export\s+)?enum\s+(\w+)/,
  ],
  python: [
    /^class\s+(\w+)/,
    /^(?:async\s+)?def\s+(\w+)/,
  ],
  ruby: [
    /^class\s+(\w+)/,
    /^module\s+(\w+)/,
    /^def\s+(\w+)/,
  ],
  go: [
    /^func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)/,
    /^type\s+(\w+)\s+struct/,
    /^type\s+(\w+)\s+interface/,
  ],
  rust: [
    /^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)/,
    /^(?:pub\s+)?struct\s+(\w+)/,
    /^(?:pub\s+)?enum\s+(\w+)/,
    /^(?:pub\s+)?trait\s+(\w+)/,
    /^impl(?:<[^>]+>)?\s+(\w+)/,
  ],
  java: [
    /^(?:public|private|protected)?\s*(?:static\s+)?(?:abstract\s+)?class\s+(\w+)/,
    /^(?:public|private|protected)?\s*interface\s+(\w+)/,
    /^(?:public|private|protected)?\s*(?:static\s+)?(?:synchronized\s+)?\w+\s+(\w+)\s*\(/,
  ],
  swift: [
    /^(?:public\s+|private\s+|internal\s+|open\s+)?(?:final\s+)?class\s+(\w+)/,
    /^(?:public\s+|private\s+|internal\s+)?struct\s+(\w+)/,
    /^(?:public\s+|private\s+|internal\s+)?protocol\s+(\w+)/,
    /^(?:public\s+|private\s+|internal\s+)?(?:static\s+)?func\s+(\w+)/,
    /^(?:public\s+|private\s+|internal\s+)?enum\s+(\w+)/,
  ],
  elixir: [
    /^defmodule\s+(\w+(?:\.\w+)*)/,
    /^(?:def|defp)\s+(\w+)/,
  ],
};

const LANGUAGE_FAMILY: Record<string, string> = {
  typescript: 'clike', javascript: 'clike', csharp: 'clike', php: 'clike',
  dart: 'clike', kotlin: 'clike', scala: 'clike',
  python: 'python',
  ruby: 'ruby',
  go: 'go',
  rust: 'rust',
  java: 'java',
  swift: 'swift',
  elixir: 'elixir', erlang: 'elixir',
  cpp: 'clike', c: 'clike',
};

const IMPORT_PATTERNS: RegExp[] = [
  /import\s+.*?from\s+['"]([^'"]+)['"]/g,        // ES import
  /import\s+['"]([^'"]+)['"]/g,                    // side-effect import
  /require\s*\(\s*['"]([^'"]+)['"]\s*\)/g,          // CommonJS
  /from\s+(\S+)\s+import/g,                         // Python
  /import\s+"([^"]+)"/g,                             // Go
  /use\s+(\w+(?:::\w+)*)/g,                          // Rust
];

export class Chunker {
  private family: string;

  constructor(private language: string) {
    this.family = LANGUAGE_FAMILY[language] || 'unknown';
  }

  chunk(code: string, filePath: string): RawChunk[] {
    if (CONFIG_LANGUAGES.has(this.language)) {
      return [this.wholeFileChunk(code, filePath, 'config')];
    }

    if (SCRIPT_LANGUAGES.has(this.language) || this.family === 'unknown') {
      const declarations = this.findDeclarations(code);
      if (declarations.length === 0) {
        return [this.wholeFileChunk(code, filePath, 'script')];
      }
    }

    const declarations = this.findDeclarations(code);
    if (declarations.length === 0) {
      return [this.wholeFileChunk(code, filePath, 'module')];
    }

    return this.splitByDeclarations(code, declarations, filePath);
  }

  private findDeclarations(code: string): Array<{
    name: string;
    kind: RawChunk['kind'];
    lineIndex: number;
  }> {
    const patterns = DECLARATION_PATTERNS[this.family];
    if (!patterns) return [];

    const lines = code.split('\n');
    const results: Array<{ name: string; kind: RawChunk['kind']; lineIndex: number }> = [];

    for (let i = 0; i < lines.length; i++) {
      const trimmed = lines[i].trimStart();
      for (const pattern of patterns) {
        const match = trimmed.match(pattern);
        if (match && match[1]) {
          const kind = this.inferKind(trimmed);
          results.push({ name: match[1], kind, lineIndex: i });
          break;
        }
      }
    }

    return results;
  }

  private inferKind(line: string): RawChunk['kind'] {
    if (/\bclass\b/.test(line)) return 'class';
    if (/\binterface\b/.test(line)) return 'interface';
    if (/\btype\b.*=/.test(line)) return 'type';
    if (/\b(enum|struct|trait|protocol)\b/.test(line)) return 'type';
    if (/\b(module|defmodule)\b/.test(line)) return 'module';
    return 'function';
  }

  private splitByDeclarations(
    code: string,
    declarations: Array<{ name: string; kind: RawChunk['kind']; lineIndex: number }>,
    filePath: string,
  ): RawChunk[] {
    const lines = code.split('\n');
    const fileImports = this.extractImports(code);
    const chunks: RawChunk[] = [];

    for (let i = 0; i < declarations.length; i++) {
      const decl = declarations[i];
      const startLine = decl.lineIndex;
      const endLine = i + 1 < declarations.length
        ? declarations[i + 1].lineIndex - 1
        : lines.length - 1;

      // Trim trailing blank lines
      let actualEnd = endLine;
      while (actualEnd > startLine && lines[actualEnd].trim() === '') {
        actualEnd--;
      }

      const chunkCode = lines.slice(startLine, actualEnd + 1).join('\n');
      const isExported = /^export\s/.test(lines[startLine].trimStart()) ||
                          /^pub\s/.test(lines[startLine].trimStart());

      chunks.push({
        name: decl.name,
        kind: decl.kind,
        lines: [startLine + 1, actualEnd + 1], // 1-indexed
        code: chunkCode,
        imports: fileImports, // all imports shared across chunks in the file
        exports: isExported ? [decl.name] : [],
      });
    }

    return chunks;
  }

  private extractImports(code: string): string[] {
    const imports = new Set<string>();
    for (const pattern of IMPORT_PATTERNS) {
      const re = new RegExp(pattern.source, pattern.flags);
      let match;
      while ((match = re.exec(code)) !== null) {
        imports.add(match[1]);
      }
    }
    return [...imports];
  }

  private wholeFileChunk(code: string, filePath: string, kind: RawChunk['kind']): RawChunk {
    const lines = code.split('\n');
    return {
      name: basename(filePath),
      kind,
      lines: [1, lines.length],
      code,
      imports: this.extractImports(code),
      exports: [],
    };
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/chunker.test.ts
```

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: language-agnostic code chunker with heuristic declaration splitting"
```

---

## Task 5: LLM Client Wrapper

**Files:**
- Create: `src/llm.ts`
- Create: `src/llm.test.ts`

Wraps the Anthropic SDK with concurrency control, retries, and structured output parsing.

**Step 1: Write failing tests**

```typescript
// src/llm.test.ts
import { describe, it, expect, vi } from 'vitest';
import { LlmClient } from './llm.js';

describe('LlmClient', () => {
  it('respects max concurrency', async () => {
    let activeCalls = 0;
    let maxActive = 0;

    const mockClient = {
      messages: {
        create: vi.fn(async () => {
          activeCalls++;
          maxActive = Math.max(maxActive, activeCalls);
          await new Promise(r => setTimeout(r, 50));
          activeCalls--;
          return {
            content: [{ type: 'text', text: '{"summary": "test"}' }],
          };
        }),
      },
    };

    const llm = new LlmClient(mockClient as any, { maxConcurrent: 2 });

    await Promise.all([
      llm.complete('prompt1', 'haiku'),
      llm.complete('prompt2', 'haiku'),
      llm.complete('prompt3', 'haiku'),
      llm.complete('prompt4', 'haiku'),
    ]);

    expect(maxActive).toBeLessThanOrEqual(2);
    expect(mockClient.messages.create).toHaveBeenCalledTimes(4);
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/llm.test.ts
```

**Step 3: Implement llm.ts**

```typescript
// src/llm.ts
import Anthropic from '@anthropic-ai/sdk';

interface LlmOptions {
  maxConcurrent?: number;
  haikuModel?: string;
  opusModel?: string;
}

type ModelTier = 'haiku' | 'opus';

export class LlmClient {
  private semaphore: number;
  private queue: Array<() => void> = [];
  private active = 0;
  private haikuModel: string;
  private opusModel: string;

  constructor(
    private client: Anthropic,
    options: LlmOptions = {},
  ) {
    this.semaphore = options.maxConcurrent ?? 5;
    this.haikuModel = options.haikuModel ?? 'claude-haiku-4-5-20251001';
    this.opusModel = options.opusModel ?? 'claude-opus-4-6';
  }

  async complete(
    prompt: string,
    tier: ModelTier,
    options?: { system?: string; maxTokens?: number },
  ): Promise<string> {
    await this.acquire();
    try {
      const model = tier === 'haiku' ? this.haikuModel : this.opusModel;
      const response = await this.client.messages.create({
        model,
        max_tokens: options?.maxTokens ?? 4096,
        system: options?.system,
        messages: [{ role: 'user', content: prompt }],
      });

      const textBlock = response.content.find(b => b.type === 'text');
      return textBlock?.text ?? '';
    } finally {
      this.release();
    }
  }

  async completeJson<T>(
    prompt: string,
    tier: ModelTier,
    options?: { system?: string; maxTokens?: number },
  ): Promise<T> {
    const text = await this.complete(prompt, tier, options);
    // Extract JSON from markdown code fences if present
    const jsonMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/);
    const jsonStr = jsonMatch ? jsonMatch[1].trim() : text.trim();
    return JSON.parse(jsonStr);
  }

  private acquire(): Promise<void> {
    if (this.active < this.semaphore) {
      this.active++;
      return Promise.resolve();
    }
    return new Promise<void>(resolve => {
      this.queue.push(() => {
        this.active++;
        resolve();
      });
    });
  }

  private release(): void {
    this.active--;
    const next = this.queue.shift();
    if (next) next();
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/llm.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: LLM client wrapper with concurrency control and JSON parsing"
```

---

## Task 6: Layer 1 Analyzer - Unit Summaries

**Files:**
- Create: `src/analyzers/layer1.ts`
- Create: `src/analyzers/layer1.test.ts`

**Step 1: Write failing tests**

```typescript
// src/analyzers/layer1.test.ts
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
      lines: [10, 25],
      code: 'export function validateToken(req) { return verify(req.headers.auth); }',
      imports: ['jsonwebtoken'],
      exports: ['validateToken'],
    };

    const unit = await analyzer.analyzeUnit(chunk, 'src/auth/middleware.ts', 'typescript');

    expect(unit.id).toBe('src/auth/middleware.ts::validateToken');
    expect(unit.summary).toContain('JWT');
    expect(unit.layer).toBe(1);
    expect(mockLlm.complete).toHaveBeenCalledTimes(1);
    // Should use haiku tier
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
      lines: [1, 3],
      code: 'function add(a, b) { return a + b; }',
      imports: [],
      exports: [],
    };

    await analyzer.analyzeUnit(chunk, 'utils.ts', 'typescript');

    expect(capturedPrompt).toContain('function add');
    expect(capturedPrompt).toContain('utils.ts');
    expect(capturedPrompt).toContain('typescript');
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/analyzers/layer1.test.ts
```

**Step 3: Implement layer1.ts**

```typescript
// src/analyzers/layer1.ts
import type { LlmClient } from '../llm.js';
import type { RawChunk } from '../chunker.js';
import type { CodeUnit } from '../types.js';

export class Layer1Analyzer {
  constructor(private llm: LlmClient) {}

  async analyzeUnit(chunk: RawChunk, filePath: string, language: string): Promise<CodeUnit> {
    const prompt = `You are analyzing a ${language} code unit from the file "${filePath}".

Code:
\`\`\`${language}
${chunk.code}
\`\`\`

Summarize what this ${chunk.kind} "${chunk.name}" does in 1-3 concise sentences. Focus on:
- What it does (purpose)
- Key inputs and outputs
- Any notable patterns, side effects, or concerns

Reply with ONLY the summary text, no prefixes or formatting.`;

    const summary = await this.llm.complete(prompt, 'haiku', {
      system: 'You are a code analyst. Provide concise, accurate summaries of code units. No markdown formatting, just plain text.',
      maxTokens: 300,
    });

    return {
      id: `${filePath}::${chunk.name}`,
      layer: 1,
      path: filePath,
      lines: chunk.lines,
      language,
      kind: chunk.kind,
      name: chunk.name,
      summary: summary.trim(),
      exports: chunk.exports,
      imports: chunk.imports,
      rawCode: chunk.code,
    };
  }

  async analyzeFile(
    chunks: RawChunk[],
    filePath: string,
    language: string,
  ): Promise<CodeUnit[]> {
    return Promise.all(
      chunks.map(chunk => this.analyzeUnit(chunk, filePath, language)),
    );
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/analyzers/layer1.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: Layer 1 analyzer - Haiku-powered unit summaries"
```

---

## Task 7: Layers 2-4 Analyzer - Opus Deep Analysis

**Files:**
- Create: `src/analyzers/deep.ts`
- Create: `src/analyzers/deep.test.ts`

This is the core intelligence. For codebases that fit within the context window, Layers 2+3+4 are produced in a single Opus call. For larger codebases, it batches.

**Step 1: Write failing tests**

```typescript
// src/analyzers/deep.test.ts
import { describe, it, expect, vi } from 'vitest';
import { DeepAnalyzer } from './deep.js';
import type { CodeUnit } from '../types.js';

const MOCK_OPUS_RESPONSE = JSON.stringify({
  relationships: [
    {
      type: 'calls',
      from: 'src/auth/middleware.ts::validateToken',
      to: 'src/auth/jwt.ts::verifyJWT',
      description: 'Delegates JWT verification',
    },
  ],
  domains: [
    {
      domain: 'Authentication',
      description: 'Handles user authentication and token management',
      units: ['src/auth/middleware.ts::validateToken', 'src/auth/jwt.ts::verifyJWT'],
      entryPoints: ['src/auth/middleware.ts::validateToken'],
      dataFlow: 'Request -> validateToken -> verifyJWT',
      concerns: ['No token refresh mechanism'],
    },
  ],
  system: {
    overview: 'A web API with JWT authentication',
    architecture: 'Layered architecture with middleware-based auth',
    domainInteractions: 'Auth domain guards all API routes',
    entryPoints: ['src/index.ts'],
    techStack: ['TypeScript', 'Express', 'JWT'],
    risks: ['Single point of failure in auth middleware'],
  },
  patterns: {
    naming: [{ rule: 'camelCase for functions', examples: ['validateToken', 'verifyJWT'], confidence: 'high' }],
    fileOrganization: [{ rule: 'Domain-based directory structure', examples: ['src/auth/'], confidence: 'high' }],
    architecture: [],
    imports: [],
    errorHandling: [],
    testing: [],
    domainBoundaries: [],
  },
});

describe('DeepAnalyzer', () => {
  it('produces relationships, domains, system narrative, and patterns from units', async () => {
    const mockLlm = {
      complete: vi.fn(async () => MOCK_OPUS_RESPONSE),
    };

    const analyzer = new DeepAnalyzer(mockLlm as any);

    const units: CodeUnit[] = [
      {
        id: 'src/auth/middleware.ts::validateToken',
        layer: 1, path: 'src/auth/middleware.ts', lines: [1, 10],
        language: 'typescript', kind: 'function', name: 'validateToken',
        summary: 'Validates JWT tokens', exports: ['validateToken'],
        imports: ['./jwt'], rawCode: 'function validateToken() {}',
      },
      {
        id: 'src/auth/jwt.ts::verifyJWT',
        layer: 1, path: 'src/auth/jwt.ts', lines: [1, 8],
        language: 'typescript', kind: 'function', name: 'verifyJWT',
        summary: 'Verifies JWT signature', exports: ['verifyJWT'],
        imports: ['jsonwebtoken'], rawCode: 'function verifyJWT() {}',
      },
    ];

    const result = await analyzer.analyze(units);

    expect(result.relationships).toHaveLength(1);
    expect(result.relationships[0].type).toBe('calls');
    expect(result.domains).toHaveLength(1);
    expect(result.domains[0].domain).toBe('Authentication');
    expect(result.system.overview).toContain('JWT');
    expect(result.patterns.naming).toHaveLength(1);
    // Should use opus tier
    expect(mockLlm.complete).toHaveBeenCalledWith(
      expect.any(String),
      'opus',
      expect.any(Object),
    );
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/analyzers/deep.test.ts
```

**Step 3: Implement deep.ts**

```typescript
// src/analyzers/deep.ts
import type { LlmClient } from '../llm.js';
import type { CodeUnit, Relationship, Domain, SystemNarrative, CodebasePatterns } from '../types.js';

export interface DeepAnalysisResult {
  relationships: Relationship[];
  domains: Domain[];
  system: SystemNarrative;
  patterns: CodebasePatterns;
}

export class DeepAnalyzer {
  constructor(private llm: LlmClient) {}

  async analyze(units: CodeUnit[]): Promise<DeepAnalysisResult> {
    const unitSummaries = units.map(u =>
      `[${u.id}] (${u.kind}, ${u.language})\n  Path: ${u.path}\n  Summary: ${u.summary}\n  Imports: ${u.imports.join(', ') || 'none'}\n  Exports: ${u.exports.join(', ') || 'none'}\n  Code:\n${u.rawCode}`
    ).join('\n\n---\n\n');

    const prompt = `You are an expert software architect performing deep analysis of a codebase. Below are all the code units discovered in the project, each with its summary, imports, exports, and source code.

Analyze this codebase and produce a JSON response with these four sections:

1. **relationships** - Cross-unit dependencies and interactions. For each relationship:
   - "type": one of "calls", "imports", "implements", "extends", "configures", "uses"
   - "from": the unit ID that depends on the other
   - "to": the unit ID being depended on
   - "description": one sentence explaining the relationship

2. **domains** - Logical bounded contexts / feature areas. Group related units:
   - "domain": short name (e.g., "Authentication", "Payment Processing")
   - "description": 2-3 sentences describing the domain's responsibility
   - "units": array of unit IDs belonging to this domain
   - "entryPoints": unit IDs that serve as entry points to this domain
   - "dataFlow": one sentence describing how data flows through the domain
   - "concerns": array of potential issues, tech debt, or risks

3. **system** - High-level system narrative:
   - "overview": what the application does (2-3 sentences)
   - "architecture": architectural patterns used (2-3 sentences)
   - "domainInteractions": how domains interact with each other
   - "entryPoints": main application entry points (file paths)
   - "techStack": detected technologies and frameworks
   - "risks": system-level concerns or risks

4. **patterns** - Coding conventions and patterns observed:
   - "naming": naming convention rules with examples
   - "fileOrganization": file/directory organization rules
   - "architecture": architectural pattern rules
   - "imports": import convention rules
   - "errorHandling": error handling pattern rules
   - "testing": testing pattern rules
   - "domainBoundaries": domain boundary rules

   Each pattern rule should have: "rule" (string), "examples" (string[]), "confidence" ("high" | "medium" | "low")

CODE UNITS:

${unitSummaries}

Respond with ONLY valid JSON, no markdown fences or commentary.`;

    const response = await this.llm.complete(prompt, 'opus', {
      system: 'You are an expert software architect. Analyze codebases with precision. Output only valid JSON.',
      maxTokens: 16384,
    });

    return this.parseResponse(response);
  }

  private parseResponse(response: string): DeepAnalysisResult {
    // Try to extract JSON from code fences if present
    const jsonMatch = response.match(/```(?:json)?\s*([\s\S]*?)```/);
    const jsonStr = jsonMatch ? jsonMatch[1].trim() : response.trim();

    const parsed = JSON.parse(jsonStr);

    // Normalize the response to match our types
    return {
      relationships: (parsed.relationships || []).map((r: any) => ({
        layer: 2 as const,
        type: r.type,
        from: r.from,
        to: r.to,
        description: r.description,
      })),
      domains: (parsed.domains || []).map((d: any) => ({
        layer: 3 as const,
        domain: d.domain,
        description: d.description,
        units: d.units || [],
        entryPoints: d.entryPoints || d.entry_points || [],
        dataFlow: d.dataFlow || d.data_flow || '',
        concerns: d.concerns || [],
      })),
      system: {
        layer: 4 as const,
        overview: parsed.system?.overview || '',
        architecture: parsed.system?.architecture || '',
        domainInteractions: parsed.system?.domainInteractions || parsed.system?.domain_interactions || '',
        entryPoints: parsed.system?.entryPoints || parsed.system?.entry_points || [],
        techStack: parsed.system?.techStack || parsed.system?.tech_stack || [],
        risks: parsed.system?.risks || [],
      },
      patterns: {
        naming: parsed.patterns?.naming || [],
        fileOrganization: parsed.patterns?.fileOrganization || parsed.patterns?.file_organization || [],
        architecture: parsed.patterns?.architecture || [],
        imports: parsed.patterns?.imports || [],
        errorHandling: parsed.patterns?.errorHandling || parsed.patterns?.error_handling || [],
        testing: parsed.patterns?.testing || [],
        domainBoundaries: parsed.patterns?.domainBoundaries || parsed.patterns?.domain_boundaries || [],
      },
    };
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/analyzers/deep.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: Layers 2-4 deep analyzer - Opus-powered relationships, domains, system narrative, and patterns"
```

---

## Task 8: FAISS Storage Layer

**Files:**
- Create: `src/storage.ts`
- Create: `src/storage.test.ts`

Handles serializing all layers into FAISS entries with proper source tagging for filtering.

**Step 1: Write failing tests**

```typescript
// src/storage.test.ts
import { describe, it, expect, vi } from 'vitest';
import { CodexStorage } from './storage.js';
import type { CodeUnit, Relationship, Domain, SystemNarrative, CodebasePatterns } from './types.js';

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
      layer: 1, path: 'src/auth.ts', lines: [1, 10],
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

  it('uses correct source prefix per layer', async () => {
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

    // Should have called deleteBySource for layers 2, 3, 4, and patterns
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:2');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:3');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/layer:4');
    expect(mockClient.deleteBySource).toHaveBeenCalledWith('codex/test-proj/patterns');
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/storage.test.ts
```

**Step 3: Implement storage.ts**

```typescript
// src/storage.ts
import type { FaissClient } from './faiss-client.js';
import type { DirectoryNode, CodeUnit } from './types.js';
import type { DeepAnalysisResult } from './analyzers/deep.js';

export class CodexStorage {
  private prefix: string;

  constructor(
    private client: FaissClient,
    projectName: string,
  ) {
    this.prefix = `codex/${projectName}`;
  }

  async storeLayer0(directories: DirectoryNode[]): Promise<void> {
    await this.client.deleteBySource(`${this.prefix}/layer:0`);

    const memories = directories.map(dir => ({
      text: this.formatDirectory(dir),
      source: `${this.prefix}/layer:0`,
      metadata: { layer: 0, path: dir.path, role: dir.role },
    }));

    await this.client.addBatch(memories);
  }

  async storeLayer1(units: CodeUnit[]): Promise<void> {
    await this.client.deleteBySource(`${this.prefix}/layer:1`);

    const memories = units.map(unit => ({
      text: this.formatUnit(unit),
      source: `${this.prefix}/layer:1`,
      metadata: {
        layer: 1,
        unitId: unit.id,
        path: unit.path,
        language: unit.language,
        kind: unit.kind,
        name: unit.name,
      },
    }));

    await this.client.addBatch(memories);
  }

  async storeDeepAnalysis(result: DeepAnalysisResult): Promise<void> {
    // Layer 2 - Relationships
    await this.client.deleteBySource(`${this.prefix}/layer:2`);
    if (result.relationships.length > 0) {
      const relMemories = result.relationships.map(rel => ({
        text: `Relationship: ${rel.from} ${rel.type} ${rel.to}. ${rel.description}`,
        source: `${this.prefix}/layer:2`,
        metadata: { layer: 2, type: rel.type, from: rel.from, to: rel.to },
      }));
      await this.client.addBatch(relMemories);
    }

    // Layer 3 - Domains
    await this.client.deleteBySource(`${this.prefix}/layer:3`);
    if (result.domains.length > 0) {
      const domainMemories = result.domains.map(domain => ({
        text: this.formatDomain(domain),
        source: `${this.prefix}/layer:3`,
        metadata: { layer: 3, domain: domain.domain },
      }));
      await this.client.addBatch(domainMemories);
    }

    // Layer 4 - System
    await this.client.deleteBySource(`${this.prefix}/layer:4`);
    const systemMemory = {
      text: this.formatSystem(result.system),
      source: `${this.prefix}/layer:4`,
      metadata: { layer: 4 },
    };
    await this.client.addBatch([systemMemory]);

    // Patterns
    await this.client.deleteBySource(`${this.prefix}/patterns`);
    const patternText = this.formatPatterns(result.patterns);
    if (patternText.trim()) {
      await this.client.addBatch([{
        text: patternText,
        source: `${this.prefix}/patterns`,
        metadata: { layer: 'patterns' },
      }]);
    }
  }

  private formatDirectory(dir: DirectoryNode): string {
    const fileList = dir.files.map(f => `  ${f.path} (${f.language}, ${f.size}b${f.isEntryPoint ? ', ENTRY POINT' : ''})`).join('\n');
    return `Directory: ${dir.path} [${dir.role}]\nFiles:\n${fileList || '  (empty)'}`;
  }

  private formatUnit(unit: CodeUnit): string {
    return `[${unit.id}] ${unit.kind} "${unit.name}" in ${unit.path}:${unit.lines[0]}-${unit.lines[1]} (${unit.language})

Summary: ${unit.summary}

Imports: ${unit.imports.join(', ') || 'none'}
Exports: ${unit.exports.join(', ') || 'none'}

Code:
${unit.rawCode}`;
  }

  private formatDomain(domain: import('./types.js').Domain): string {
    return `Domain: ${domain.domain}

${domain.description}

Units: ${domain.units.join(', ')}
Entry Points: ${domain.entryPoints.join(', ')}
Data Flow: ${domain.dataFlow}
Concerns: ${domain.concerns.length > 0 ? domain.concerns.join('; ') : 'none identified'}`;
  }

  private formatSystem(system: import('./types.js').SystemNarrative): string {
    return `System Overview:
${system.overview}

Architecture:
${system.architecture}

Domain Interactions:
${system.domainInteractions}

Entry Points: ${system.entryPoints.join(', ')}
Tech Stack: ${system.techStack.join(', ')}
Risks: ${system.risks.length > 0 ? system.risks.join('; ') : 'none identified'}`;
  }

  private formatPatterns(patterns: import('./types.js').CodebasePatterns): string {
    const sections: string[] = [];
    const format = (name: string, rules: import('./types.js').PatternRule[]) => {
      if (rules.length === 0) return;
      sections.push(`${name}:\n${rules.map(r => `- ${r.rule} (${r.confidence}) e.g. ${r.examples.join(', ')}`).join('\n')}`);
    };

    format('Naming', patterns.naming);
    format('File Organization', patterns.fileOrganization);
    format('Architecture', patterns.architecture);
    format('Imports', patterns.imports);
    format('Error Handling', patterns.errorHandling);
    format('Testing', patterns.testing);
    format('Domain Boundaries', patterns.domainBoundaries);

    return sections.join('\n\n');
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/storage.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: FAISS storage layer - serialize all layers into tagged memories"
```

---

## Task 9: Pattern Enforcement Skill Generator

**Files:**
- Create: `src/skill-generator.ts`
- Create: `src/skill-generator.test.ts`

Generates CLAUDE.md and .cursorrules files from discovered patterns.

**Step 1: Write failing tests**

```typescript
// src/skill-generator.test.ts
import { describe, it, expect } from 'vitest';
import { SkillGenerator } from './skill-generator.js';
import type { CodebasePatterns, SystemNarrative, Domain } from './types.js';

describe('SkillGenerator', () => {
  const patterns: CodebasePatterns = {
    naming: [
      { rule: 'camelCase for functions and variables', examples: ['validateToken', 'getUserById'], confidence: 'high' },
      { rule: 'PascalCase for classes and interfaces', examples: ['UserService', 'AuthMiddleware'], confidence: 'high' },
    ],
    fileOrganization: [
      { rule: 'Domain-based directory structure under src/', examples: ['src/auth/', 'src/users/'], confidence: 'high' },
    ],
    architecture: [
      { rule: 'All DB access through repository classes', examples: ['UserRepository.findById()'], confidence: 'medium' },
    ],
    imports: [
      { rule: 'Absolute imports from @/ prefix', examples: ['@/utils/validate'], confidence: 'high' },
    ],
    errorHandling: [
      { rule: 'API handlers wrap in try/catch with standardized error response', examples: [], confidence: 'medium' },
    ],
    testing: [
      { rule: 'Co-located test files with .test.ts suffix', examples: ['auth.test.ts'], confidence: 'high' },
    ],
    domainBoundaries: [
      { rule: 'Auth domain must not import directly from Payment domain', examples: [], confidence: 'medium' },
    ],
  };

  const system: SystemNarrative = {
    layer: 4,
    overview: 'A REST API for user management',
    architecture: 'Layered architecture with Express middleware',
    domainInteractions: 'Auth guards all routes',
    entryPoints: ['src/index.ts'],
    techStack: ['TypeScript', 'Express', 'PostgreSQL'],
    risks: [],
  };

  it('generates CLAUDE.md with pattern rules', () => {
    const generator = new SkillGenerator();
    const output = generator.generateClaudeMd(patterns, system, []);

    expect(output).toContain('camelCase for functions');
    expect(output).toContain('PascalCase for classes');
    expect(output).toContain('Domain-based directory');
    expect(output).toContain('repository classes');
    expect(output).toContain('# Codebase Intelligence');
  });

  it('generates .cursorrules', () => {
    const generator = new SkillGenerator();
    const output = generator.generateCursorRules(patterns, system, []);

    expect(output).toContain('camelCase');
    expect(output).toContain('Domain-based');
  });

  it('only includes high-confidence rules by default', () => {
    const generator = new SkillGenerator();
    const output = generator.generateClaudeMd(patterns, system, [], { minConfidence: 'high' });

    expect(output).toContain('camelCase');
    expect(output).not.toContain('repository classes'); // medium confidence
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/skill-generator.test.ts
```

**Step 3: Implement skill-generator.ts**

```typescript
// src/skill-generator.ts
import type { CodebasePatterns, SystemNarrative, Domain, PatternRule } from './types.js';

interface GenerateOptions {
  minConfidence?: 'high' | 'medium' | 'low';
}

export class SkillGenerator {
  generateClaudeMd(
    patterns: CodebasePatterns,
    system: SystemNarrative,
    domains: Domain[],
    options: GenerateOptions = {},
  ): string {
    const minConfidence = options.minConfidence ?? 'medium';
    const filter = (rules: PatternRule[]) => this.filterByConfidence(rules, minConfidence);

    const sections: string[] = [];

    sections.push('# Codebase Intelligence (Auto-generated by Codex)');
    sections.push('');
    sections.push('> This file was auto-generated by Codex codebase indexer. It encodes discovered patterns and conventions.');
    sections.push('> Re-run `codex patterns .` to update after significant codebase changes.');
    sections.push('');

    // System overview
    sections.push('## System Overview');
    sections.push('');
    sections.push(system.overview);
    sections.push('');
    sections.push(`**Architecture:** ${system.architecture}`);
    sections.push(`**Tech Stack:** ${system.techStack.join(', ')}`);
    sections.push(`**Entry Points:** ${system.entryPoints.join(', ')}`);
    sections.push('');

    // Domains
    if (domains.length > 0) {
      sections.push('## Domains');
      sections.push('');
      for (const domain of domains) {
        sections.push(`### ${domain.domain}`);
        sections.push('');
        sections.push(domain.description);
        sections.push(`- **Entry Points:** ${domain.entryPoints.join(', ')}`);
        sections.push(`- **Data Flow:** ${domain.dataFlow}`);
        if (domain.concerns.length > 0) {
          sections.push(`- **Known Concerns:** ${domain.concerns.join('; ')}`);
        }
        sections.push('');
      }
    }

    // Conventions
    sections.push('## Coding Conventions');
    sections.push('');
    sections.push('Follow these patterns when writing new code:');
    sections.push('');

    this.addPatternSection(sections, 'Naming', filter(patterns.naming));
    this.addPatternSection(sections, 'File Organization', filter(patterns.fileOrganization));
    this.addPatternSection(sections, 'Architecture', filter(patterns.architecture));
    this.addPatternSection(sections, 'Imports', filter(patterns.imports));
    this.addPatternSection(sections, 'Error Handling', filter(patterns.errorHandling));
    this.addPatternSection(sections, 'Testing', filter(patterns.testing));
    this.addPatternSection(sections, 'Domain Boundaries', filter(patterns.domainBoundaries));

    return sections.join('\n');
  }

  generateCursorRules(
    patterns: CodebasePatterns,
    system: SystemNarrative,
    domains: Domain[],
    options: GenerateOptions = {},
  ): string {
    const minConfidence = options.minConfidence ?? 'medium';
    const filter = (rules: PatternRule[]) => this.filterByConfidence(rules, minConfidence);

    const lines: string[] = [];
    lines.push(`# Project: ${system.overview.split('.')[0]}`);
    lines.push('');
    lines.push(`Tech Stack: ${system.techStack.join(', ')}`);
    lines.push(`Architecture: ${system.architecture}`);
    lines.push('');
    lines.push('## Rules');
    lines.push('');

    const allRules = [
      ...filter(patterns.naming),
      ...filter(patterns.fileOrganization),
      ...filter(patterns.architecture),
      ...filter(patterns.imports),
      ...filter(patterns.errorHandling),
      ...filter(patterns.testing),
      ...filter(patterns.domainBoundaries),
    ];

    for (const rule of allRules) {
      lines.push(`- ${rule.rule}`);
    }

    if (domains.length > 0) {
      lines.push('');
      lines.push('## Domains');
      for (const d of domains) {
        lines.push(`- ${d.domain}: ${d.description.split('.')[0]}`);
      }
    }

    return lines.join('\n');
  }

  private addPatternSection(sections: string[], title: string, rules: PatternRule[]): void {
    if (rules.length === 0) return;

    sections.push(`### ${title}`);
    sections.push('');
    for (const rule of rules) {
      const examples = rule.examples.length > 0 ? ` (e.g., ${rule.examples.join(', ')})` : '';
      sections.push(`- ${rule.rule}${examples}`);
    }
    sections.push('');
  }

  private filterByConfidence(rules: PatternRule[], min: 'high' | 'medium' | 'low'): PatternRule[] {
    const levels = { high: 3, medium: 2, low: 1 };
    const minLevel = levels[min];
    return rules.filter(r => levels[r.confidence] >= minLevel);
  }
}
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/skill-generator.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: pattern enforcement skill generator - CLAUDE.md and .cursorrules output"
```

---

## Task 10: Index Manifest & Incremental Updates

**Files:**
- Create: `src/manifest.ts`
- Create: `src/manifest.test.ts`

**Step 1: Write failing tests**

```typescript
// src/manifest.test.ts
import { describe, it, expect } from 'vitest';
import { IndexManifest } from './manifest.js';
import { mkdtempSync, writeFileSync, mkdirSync } from 'fs';
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
    mkdirSync(join(tmp, '.codex'), { recursive: true });

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
    mkdirSync(join(tmp, '.codex'), { recursive: true });

    const manifest = new IndexManifest(tmp, 'test-project');
    manifest.updateFileHash('src/a.ts', 'hash1');
    manifest.updateFileHash('src/b.ts', 'hash2');
    manifest.save();

    // Simulate changes
    const changed = manifest.getChangedFiles({
      'src/a.ts': 'hash1',     // unchanged
      'src/b.ts': 'hash_new',  // changed
      'src/c.ts': 'hash3',     // new
    });

    expect(changed.modified).toContain('src/b.ts');
    expect(changed.added).toContain('src/c.ts');
    expect(changed.unchanged).toContain('src/a.ts');
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/manifest.test.ts
```

**Step 3: Implement manifest.ts**

```typescript
// src/manifest.ts
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
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/manifest.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: index manifest for tracking file hashes and incremental updates"
```

---

## Task 11: Pipeline Orchestrator

**Files:**
- Create: `src/pipeline.ts`
- Create: `src/pipeline.test.ts`

Wires everything together: scanner → chunker → Layer 1 → Opus deep analysis → FAISS storage → skill generation.

**Step 1: Write failing test**

```typescript
// src/pipeline.test.ts
import { describe, it, expect, vi } from 'vitest';
import { Pipeline } from './pipeline.js';

describe('Pipeline', () => {
  it('runs all phases in order for a full index', async () => {
    const callOrder: string[] = [];

    const mockScanner = {
      scan: vi.fn(async () => {
        callOrder.push('scan');
        return {
          rootPath: '/tmp/test',
          directories: [{ path: '.', role: 'source', files: [{ path: 'main.ts', language: 'typescript', size: 50, lastModified: '', isEntryPoint: true }], children: [] }],
          files: [{ path: 'main.ts', language: 'typescript', size: 50, lastModified: '', isEntryPoint: true }],
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
          id: 'main.ts::main', layer: 1, path: 'main.ts', lines: [1, 5] as [number, number],
          language: 'typescript', kind: 'function' as const, name: 'main',
          summary: 'Entry point', exports: [], imports: [], rawCode: '',
        }];
      }),
    };

    const mockDeep = {
      analyze: vi.fn(async () => {
        callOrder.push('deep');
        return {
          relationships: [],
          domains: [],
          system: { layer: 4 as const, overview: '', architecture: '', domainInteractions: '', entryPoints: [], techStack: [], risks: [] },
          patterns: { naming: [], fileOrganization: [], architecture: [], imports: [], errorHandling: [], testing: [], domainBoundaries: [] },
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
      projectPath: '/tmp/test',
    });

    await pipeline.runFull();

    expect(callOrder).toEqual(['scan', 'layer1', 'deep', 'store0', 'store1', 'storeDeep']);
  });
});
```

**Step 2: Run test to verify it fails**

```bash
npx vitest run src/pipeline.test.ts
```

**Step 3: Implement pipeline.ts**

```typescript
// src/pipeline.ts
import { readFileSync } from 'fs';
import { join } from 'path';
import type { Scanner, ScanResult } from './scanner.js';
import type { Chunker } from './chunker.js';
import type { Layer1Analyzer } from './analyzers/layer1.js';
import type { DeepAnalyzer, DeepAnalysisResult } from './analyzers/deep.js';
import type { CodexStorage } from './storage.js';
import type { IndexManifest } from './manifest.js';
import type { SkillGenerator } from './skill-generator.js';
import type { CodeUnit } from './types.js';
import { Chunker as ChunkerImpl } from './chunker.js';

interface PipelineComponents {
  scanner: Scanner;
  layer1: Layer1Analyzer;
  deep: DeepAnalyzer;
  storage: CodexStorage;
  manifest: IndexManifest;
  skillGenerator: SkillGenerator;
  projectPath: string;
}

export interface PipelineResult {
  scan: ScanResult;
  units: CodeUnit[];
  deepAnalysis: DeepAnalysisResult;
  filesProcessed: number;
  unitsGenerated: number;
  domainsFound: number;
}

export class Pipeline {
  constructor(private components: PipelineComponents) {}

  async runFull(): Promise<PipelineResult> {
    const { scanner, layer1, deep, storage, manifest, projectPath } = this.components;

    // Phase 1: Scan
    const scan = await scanner.scan();

    // Phase 2: Chunk + Analyze (Layer 1)
    const allUnits: CodeUnit[] = [];
    for (const file of scan.files) {
      if (file.language === 'unknown') continue;

      try {
        const content = readFileSync(join(projectPath, file.path), 'utf-8');
        const chunker = new ChunkerImpl(file.language);
        const chunks = chunker.chunk(content, file.path);
        const units = await layer1.analyzeFile(chunks, file.path, file.language);
        allUnits.push(...units);

        manifest.updateFileHash(file.path, IndexManifest.hashFile(content));
      } catch {
        // Skip files that can't be read/parsed
      }
    }

    // Phase 3: Deep Analysis (Layers 2-4)
    const deepAnalysis = await deep.analyze(allUnits);

    // Phase 4: Store everything
    await storage.storeLayer0(scan.directories);
    await storage.storeLayer1(allUnits);
    await storage.storeDeepAnalysis(deepAnalysis);

    // Phase 5: Update manifest
    manifest.updateLayerTimestamp('0');
    manifest.updateLayerTimestamp('1');
    manifest.updateLayerTimestamp('2');
    manifest.updateLayerTimestamp('3');
    manifest.updateLayerTimestamp('4');
    manifest.updateStats({
      totalFiles: scan.files.length,
      totalUnits: allUnits.length,
      totalDomains: deepAnalysis.domains.length,
      languages: scan.languages,
    });
    manifest.save();

    return {
      scan,
      units: allUnits,
      deepAnalysis,
      filesProcessed: scan.files.length,
      unitsGenerated: allUnits.length,
      domainsFound: deepAnalysis.domains.length,
    };
  }
}
```

Note: The `IndexManifest.hashFile` reference in pipeline.ts requires importing it. Add this import:

```typescript
import { IndexManifest as ManifestClass } from './manifest.js';
```

And change the `manifest.updateFileHash` line to use:
```typescript
manifest.updateFileHash(file.path, ManifestClass.hashFile(content));
```

**Step 4: Run tests to verify they pass**

```bash
npx vitest run src/pipeline.test.ts
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: pipeline orchestrator - wires scanner, analyzers, storage, and manifest"
```

---

## Task 12: Wire CLI Commands

**Files:**
- Modify: `src/index.ts`

Wire the CLI commands to the actual pipeline, query, patterns, and status implementations.

**Step 1: Update src/index.ts with full CLI wiring**

```typescript
#!/usr/bin/env node
import { Command } from 'commander';
import { resolve, basename } from 'path';
import Anthropic from '@anthropic-ai/sdk';
import chalk from 'chalk';
import ora from 'ora';
import { loadConfig } from './config.js';
import { FaissClient } from './faiss-client.js';
import { Scanner } from './scanner.js';
import { Layer1Analyzer } from './analyzers/layer1.js';
import { DeepAnalyzer } from './analyzers/deep.js';
import { CodexStorage } from './storage.js';
import { IndexManifest } from './manifest.js';
import { SkillGenerator } from './skill-generator.js';
import { LlmClient } from './llm.js';
import { Pipeline } from './pipeline.js';
import { writeFileSync } from 'fs';
import { join } from 'path';

const program = new Command();

program
  .name('codex')
  .description('Codebase intelligence - index any codebase into a layered context graph')
  .version('0.1.0');

program
  .command('index')
  .description('Index a codebase')
  .argument('<path>', 'Path to the codebase to index')
  .option('--full', 'Force full re-index')
  .action(async (targetPath: string, options) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const config = loadConfig();

    const spinner = ora('Checking FAISS connection...').start();

    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);
    const healthy = await faiss.health();
    if (!healthy) {
      spinner.fail('Cannot connect to FAISS at ' + config.faissUrl);
      process.exit(1);
    }

    if (!config.anthropicApiKey) {
      spinner.fail('ANTHROPIC_API_KEY not set');
      process.exit(1);
    }

    const anthropic = new Anthropic({ apiKey: config.anthropicApiKey });
    const llm = new LlmClient(anthropic, {
      maxConcurrent: config.maxConcurrentLlmCalls,
      haikuModel: config.haikuModel,
      opusModel: config.opusModel,
    });

    const scanner = new Scanner(absPath);
    const layer1 = new Layer1Analyzer(llm);
    const deep = new DeepAnalyzer(llm);
    const storage = new CodexStorage(faiss, projectName);
    const manifest = new IndexManifest(absPath, projectName);
    const skillGenerator = new SkillGenerator();

    if (!options.full) manifest.load();

    const pipeline = new Pipeline({
      scanner, layer1, deep, storage, manifest, skillGenerator, projectPath: absPath,
    });

    spinner.text = 'Scanning file tree...';
    const result = await pipeline.runFull();

    spinner.succeed(chalk.green('Indexing complete!'));
    console.log('');
    console.log(`  Files scanned:  ${chalk.bold(result.filesProcessed)}`);
    console.log(`  Code units:     ${chalk.bold(result.unitsGenerated)}`);
    console.log(`  Domains found:  ${chalk.bold(result.domainsFound)}`);
    console.log(`  Languages:      ${result.scan.languages.join(', ')}`);
    console.log('');
    console.log(`Run ${chalk.cyan('codex query "your question"')} to search the index.`);
    console.log(`Run ${chalk.cyan('codex patterns ' + targetPath)} to generate CLAUDE.md.`);
  });

program
  .command('query')
  .description('Query the indexed codebase')
  .argument('<query>', 'Natural language query')
  .option('--project <name>', 'Project name to search within')
  .option('-k <count>', 'Number of results', '10')
  .action(async (query: string, options) => {
    const config = loadConfig();
    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);

    const sourceFilter = options.project ? `codex/${options.project}` : 'codex/';
    const results = await faiss.search(query, {
      k: parseInt(options.k),
      source: sourceFilter,
    });

    if (results.length === 0) {
      console.log(chalk.yellow('No results found. Have you indexed this project?'));
      return;
    }

    for (const result of results) {
      const score = (result.score * 100).toFixed(1);
      console.log(chalk.dim(`[${score}%]`) + ' ' + chalk.cyan(result.source));
      console.log(result.text.slice(0, 300));
      console.log('');
    }
  });

program
  .command('patterns')
  .description('Generate pattern enforcement files')
  .argument('<path>', 'Path to the indexed codebase')
  .option('--format <format>', 'Output format: claude, cursor, all', 'all')
  .action(async (targetPath: string, options) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const config = loadConfig();
    const faiss = new FaissClient(config.faissUrl, config.faissApiKey);

    const spinner = ora('Fetching indexed patterns...').start();

    // Fetch patterns, domains, and system from FAISS
    const patternResults = await faiss.search('coding patterns conventions naming', { k: 5, source: `codex/${projectName}/patterns` });
    const domainResults = await faiss.search('domain bounded context', { k: 20, source: `codex/${projectName}/layer:3` });
    const systemResults = await faiss.search('system overview architecture', { k: 3, source: `codex/${projectName}/layer:4` });

    if (patternResults.length === 0) {
      spinner.fail('No indexed patterns found. Run `codex index` first.');
      return;
    }

    spinner.succeed('Patterns fetched');

    const generator = new SkillGenerator();

    // For now, output the raw patterns text as-is since we store them formatted
    // In a production version, we'd parse them back into structured data

    if (options.format === 'claude' || options.format === 'all') {
      const claudeMdPath = join(absPath, 'CLAUDE.md');
      const content = `# Codebase Intelligence (Auto-generated by Codex)\n\n${patternResults.map(r => r.text).join('\n\n')}\n\n## System\n\n${systemResults.map(r => r.text).join('\n\n')}\n\n## Domains\n\n${domainResults.map(r => r.text).join('\n\n')}`;
      writeFileSync(claudeMdPath, content);
      console.log(chalk.green(`  Generated ${claudeMdPath}`));
    }

    if (options.format === 'cursor' || options.format === 'all') {
      const cursorPath = join(absPath, '.cursorrules');
      const content = `# Project Conventions (Auto-generated by Codex)\n\n${patternResults.map(r => r.text).join('\n\n')}`;
      writeFileSync(cursorPath, content);
      console.log(chalk.green(`  Generated ${cursorPath}`));
    }
  });

program
  .command('status')
  .description('Show index status')
  .argument('<path>', 'Path to the codebase')
  .action(async (targetPath: string) => {
    const absPath = resolve(targetPath);
    const projectName = basename(absPath);
    const manifest = new IndexManifest(absPath, projectName);

    if (manifest.isFirstRun()) {
      console.log(chalk.yellow('This project has not been indexed yet.'));
      console.log(`Run ${chalk.cyan('codex index ' + targetPath)} to index it.`);
      return;
    }

    manifest.load();
    console.log(chalk.bold('Codex Index Status'));
    console.log('');
    console.log(`  Project: ${projectName}`);
    console.log(`  Path:    ${absPath}`);
    // Manifest data is accessed through the class
    console.log('');
    console.log(`Run ${chalk.cyan('codex index ' + targetPath)} to re-index.`);
  });

program.parse();
```

**Step 2: Verify CLI works**

```bash
npx tsx src/index.ts --help
npx tsx src/index.ts index --help
npx tsx src/index.ts status .
```

Expected: Help output displays correctly, status shows "not indexed yet".

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: wire CLI commands to pipeline, query, patterns, and status"
```

---

## Task 13: Integration Test - End to End

**Files:**
- Create: `src/integration.test.ts`

Run the full pipeline against a small test fixture with mocked LLM calls.

**Step 1: Write integration test**

```typescript
// src/integration.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
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
    const sources = new Set(storedMemories.map(m => m.source));
    expect(sources.has('codex/test-app/layer:0')).toBe(true);
    expect(sources.has('codex/test-app/layer:1')).toBe(true);

    // Verify manifest was saved
    expect(manifest.isFirstRun()).toBe(false);
  });
});
```

**Step 2: Run all tests**

```bash
npx vitest run
```

Expected: All tests pass including integration.

**Step 3: Commit**

```bash
git add -A
git commit -m "test: end-to-end integration test with mocked LLM and FAISS"
```

---

## Task 14: Final Polish & npm bin Link

**Step 1: Add bin entry and build**

```bash
npx tsc
```

Verify no TypeScript errors.

**Step 2: Test the built CLI**

```bash
node dist/index.js --help
```

**Step 3: Link locally for global usage**

```bash
npm link
```

Now `codex` is available globally.

**Step 4: Run full test suite one final time**

```bash
npx vitest run
```

**Step 5: Commit**

```bash
git add -A
git commit -m "chore: build verification and npm link setup"
```

---

## Summary

| Task | What it builds | Key files |
|------|---------------|-----------|
| 1 | Project scaffolding | package.json, tsconfig, types, CLI skeleton |
| 2 | Config + FAISS client | config.ts, faiss-client.ts |
| 3 | Layer 0 scanner | scanner.ts, languages.ts |
| 4 | Code chunker | chunker.ts |
| 5 | LLM client wrapper | llm.ts |
| 6 | Layer 1 analyzer | analyzers/layer1.ts |
| 7 | Layers 2-4 deep analyzer | analyzers/deep.ts |
| 8 | FAISS storage | storage.ts |
| 9 | Skill generator | skill-generator.ts |
| 10 | Index manifest | manifest.ts |
| 11 | Pipeline orchestrator | pipeline.ts |
| 12 | CLI wiring | index.ts (update) |
| 13 | Integration test | integration.test.ts |
| 14 | Build & link | dist/, npm link |
