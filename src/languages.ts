import { extname, basename } from 'path';

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
