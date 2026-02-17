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
        const chunker = new Chunker(file.language);
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
