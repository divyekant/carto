import { describe, it, expect } from 'vitest';
import { detectLanguage, inferDirectoryRole, isEntryPoint } from './languages.js';
import { Scanner } from './scanner.js';
import { mkdtempSync, writeFileSync, mkdirSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';

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
