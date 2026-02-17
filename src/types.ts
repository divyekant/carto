// Layer 0 - Structure
export interface DirectoryNode {
  path: string;
  role: string;
  files: FileInfo[];
  children: string[];
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
  id: string;
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
  from: string;
  to: string;
  description: string;
}

// Layer 3 - Domains
export interface Domain {
  layer: 3;
  domain: string;
  description: string;
  units: string[];
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
