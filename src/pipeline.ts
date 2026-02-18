import { readFileSync } from 'fs';
import { join } from 'path';
import type { Scanner, ScanResult } from './scanner.js';
import type { Layer1Analyzer } from './analyzers/layer1.js';
import type { DeepAnalyzer, DeepAnalysisResult } from './analyzers/deep.js';
import type { CodexStorage } from './storage.js';
import type { SkillGenerator } from './skill-generator.js';
import type { CodeUnit } from './types.js';
import { Chunker } from './chunker.js';
import { IndexManifest } from './manifest.js';

export type PipelineLog = (message: string) => void;

interface PipelineComponents {
  scanner: Scanner;
  layer1: Layer1Analyzer;
  deep: DeepAnalyzer;
  storage: CodexStorage;
  manifest: IndexManifest;
  skillGenerator: SkillGenerator;
  projectPath: string;
  log?: PipelineLog;
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
    const log = this.components.log ?? (() => {});

    // Phase 1: Scan
    log('Phase 1/5: Scanning file tree...');
    const scan = await scanner.scan();
    const codeFiles = scan.files.filter(f => f.language !== 'unknown');
    log(`  Found ${scan.files.length} files (${codeFiles.length} code files) in ${scan.directories.length} directories`);
    log(`  Languages: ${scan.languages.join(', ')}`);
    if (scan.entryPoints.length > 0) {
      log(`  Entry points: ${scan.entryPoints.join(', ')}`);
    }

    // Phase 2: Chunk + Analyze (Layer 1)
    log('Phase 2/5: Chunking & analyzing code units (Layer 1 - Haiku)...');
    const allUnits: CodeUnit[] = [];
    let processed = 0;
    let skipped = 0;
    for (const file of scan.files) {
      if (file.language === 'unknown') {
        skipped++;
        continue;
      }

      try {
        const content = readFileSync(join(projectPath, file.path), 'utf-8');
        const chunker = new Chunker(file.language);
        const chunks = chunker.chunk(content, file.path);
        const units = await layer1.analyzeFile(chunks, file.path, file.language);
        allUnits.push(...units);

        manifest.updateFileHash(file.path, IndexManifest.hashFile(content));
        processed++;
        if (processed % 10 === 0 || processed === codeFiles.length) {
          log(`  Analyzed ${processed}/${codeFiles.length} files (${allUnits.length} units so far)`);
        }
      } catch {
        skipped++;
      }
    }
    log(`  Layer 1 complete: ${allUnits.length} code units from ${processed} files (${skipped} skipped)`);

    // Phase 3: Deep Analysis (Layers 2-4)
    const totalChars = allUnits.reduce((sum, u) => sum + (u.rawCode?.length || 0), 0);
    const estTokens = Math.ceil(totalChars / 4);
    log(`Phase 3/5: Deep analysis (Layers 2-4 - Opus)...`);
    log(`  Sending ${allUnits.length} units (~${Math.round(estTokens / 1000)}k tokens) to Opus`);
    if (estTokens > 150_000) {
      log(`  Using compact mode (summaries only) to stay within context limits`);
    }
    const deepAnalysis = await deep.analyze(allUnits);
    log(`  Found ${deepAnalysis.relationships.length} relationships, ${deepAnalysis.domains.length} domains`);
    log(`  Patterns: ${Object.values(deepAnalysis.patterns).reduce((sum, arr) => sum + arr.length, 0)} rules discovered`);

    // Phase 4: Store everything
    log('Phase 4/5: Storing to FAISS...');
    log(`  Storing Layer 0 (${scan.directories.length} directories)...`);
    await storage.storeLayer0(scan.directories);
    log(`  Storing Layer 1 (${allUnits.length} units)...`);
    await storage.storeLayer1(allUnits);
    log('  Storing Layers 2-4 (relationships, domains, system, patterns)...');
    await storage.storeDeepAnalysis(deepAnalysis);
    log('  All layers stored.');

    // Phase 5: Update manifest
    log('Phase 5/5: Updating manifest...');
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
    log('  Manifest saved.');

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
