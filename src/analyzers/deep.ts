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
    const jsonMatch = response.match(/```(?:json)?\s*([\s\S]*?)```/);
    const jsonStr = jsonMatch ? jsonMatch[1].trim() : response.trim();

    const parsed = JSON.parse(jsonStr);

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
