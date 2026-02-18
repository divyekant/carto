import { readdirSync, statSync, readFileSync, existsSync } from 'fs';
import { join, basename } from 'path';
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
  'package-lock.json', 'yarn.lock', 'pnpm-lock.yaml', 'bun.lockb',
  'Gemfile.lock', 'Cargo.lock', 'composer.lock', 'poetry.lock',
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
