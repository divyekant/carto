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
    // Estimate token count (~4 chars per token) to avoid long-context rate limits.
    // If the full code would exceed ~150k tokens, send summaries only.
    const fullTexts = units.map(u =>
      `[${u.id}] (${u.kind}, ${u.language})\n  Path: ${u.path}\n  Summary: ${u.summary}\n  Imports: ${u.imports.join(', ') || 'none'}\n  Exports: ${u.exports.join(', ') || 'none'}\n  Code:\n${u.rawCode}`
    );
    const totalChars = fullTexts.reduce((sum, t) => sum + t.length, 0);
    const estimatedTokens = Math.ceil(totalChars / 4);

    let unitSummaries: string;
    if (estimatedTokens > 150_000) {
      // Use compact summaries without raw code for large codebases
      unitSummaries = units.map(u =>
        `[${u.id}] (${u.kind}, ${u.language})\n  Path: ${u.path}\n  Summary: ${u.summary}\n  Imports: ${u.imports.join(', ') || 'none'}\n  Exports: ${u.exports.join(', ') || 'none'}`
      ).join('\n\n---\n\n');
    } else {
      unitSummaries = fullTexts.join('\n\n---\n\n');
    }

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
      maxTokens: 32768,
    });

    return this.parseResponse(response);
  }

  private repairJson(json: string): string {
    // Remove trailing comma if present
    let repaired = json.replace(/,\s*$/, '');

    // Remove any incomplete last element (truncated string/object)
    // Find last complete element by looking for last complete value before truncation
    const lastComplete = Math.max(
      repaired.lastIndexOf('},'),
      repaired.lastIndexOf('}]'),
      repaired.lastIndexOf('"]'),
      repaired.lastIndexOf('"}'),
    );
    if (lastComplete > repaired.length * 0.5) {
      // Only trim if we're past the halfway point (enough data to be useful)
      repaired = repaired.slice(0, lastComplete + 1);
    }

    // Count open/close brackets and close any remaining open ones
    const opens: string[] = [];
    let inString = false;
    let escape = false;
    for (const ch of repaired) {
      if (escape) { escape = false; continue; }
      if (ch === '\\') { escape = true; continue; }
      if (ch === '"') { inString = !inString; continue; }
      if (inString) continue;
      if (ch === '{' || ch === '[') opens.push(ch);
      if (ch === '}' || ch === ']') opens.pop();
    }
    // Close any remaining opens in reverse
    for (let i = opens.length - 1; i >= 0; i--) {
      repaired += opens[i] === '{' ? '}' : ']';
    }

    return repaired;
  }

  private parseResponse(response: string): DeepAnalysisResult {
    let jsonStr = response.trim();

    // Strip markdown fences if present (greedy to capture all content between first and last fence)
    const fenceMatch = jsonStr.match(/```(?:json)?\s*\n?([\s\S]+)\n?\s*```$/);
    if (fenceMatch) {
      jsonStr = fenceMatch[1].trim();
    }

    // If it still doesn't start with {, try to find the first { and last }
    if (!jsonStr.startsWith('{')) {
      const start = jsonStr.indexOf('{');
      const end = jsonStr.lastIndexOf('}');
      if (start !== -1 && end !== -1) {
        jsonStr = jsonStr.slice(start, end + 1);
      }
    }

    let parsed: any;
    try {
      parsed = JSON.parse(jsonStr);
    } catch {
      // LLM response may be truncated — try to repair by closing open arrays/objects
      parsed = JSON.parse(this.repairJson(jsonStr));
    }

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
