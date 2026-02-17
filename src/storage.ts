import type { FaissClient } from './faiss-client.js';
import type { DirectoryNode, CodeUnit, Domain, SystemNarrative, CodebasePatterns, PatternRule } from './types.js';
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

  private formatDomain(domain: Domain): string {
    return `Domain: ${domain.domain}

${domain.description}

Units: ${domain.units.join(', ')}
Entry Points: ${domain.entryPoints.join(', ')}
Data Flow: ${domain.dataFlow}
Concerns: ${domain.concerns.length > 0 ? domain.concerns.join('; ') : 'none identified'}`;
  }

  private formatSystem(system: SystemNarrative): string {
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

  private formatPatterns(patterns: CodebasePatterns): string {
    const sections: string[] = [];
    const format = (name: string, rules: PatternRule[]) => {
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
