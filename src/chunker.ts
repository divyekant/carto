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
  /import\s+.*?from\s+['"]([^'"]+)['"]/g,
  /import\s+['"]([^'"]+)['"]/g,
  /require\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
  /from\s+(\S+)\s+import/g,
  /import\s+"([^"]+)"/g,
  /use\s+(\w+(?:::\w+)*)/g,
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

    const indentSensitive = new Set(['python', 'ruby', 'elixir']);
    const topLevelOnly = indentSensitive.has(this.family);

    const lines = code.split('\n');
    const results: Array<{ name: string; kind: RawChunk['kind']; lineIndex: number }> = [];

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      // For indent-sensitive languages, skip indented lines (methods inside classes)
      if (topLevelOnly && line.length > 0 && /^\s/.test(line)) {
        continue;
      }
      const trimmed = line.trimStart();
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
        lines: [startLine + 1, actualEnd + 1],
        code: chunkCode,
        imports: fileImports,
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
