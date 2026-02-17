import type { LlmClient } from '../llm.js';
import type { RawChunk } from '../chunker.js';
import type { CodeUnit } from '../types.js';

export class Layer1Analyzer {
  constructor(private llm: LlmClient) {}

  async analyzeUnit(chunk: RawChunk, filePath: string, language: string): Promise<CodeUnit> {
    const prompt = `You are analyzing a ${language} code unit from the file "${filePath}".

Code:
\`\`\`${language}
${chunk.code}
\`\`\`

Summarize what this ${chunk.kind} "${chunk.name}" does in 1-3 concise sentences. Focus on:
- What it does (purpose)
- Key inputs and outputs
- Any notable patterns, side effects, or concerns

Reply with ONLY the summary text, no prefixes or formatting.`;

    const summary = await this.llm.complete(prompt, 'haiku', {
      system: 'You are a code analyst. Provide concise, accurate summaries of code units. No markdown formatting, just plain text.',
      maxTokens: 300,
    });

    return {
      id: `${filePath}::${chunk.name}`,
      layer: 1,
      path: filePath,
      lines: chunk.lines,
      language,
      kind: chunk.kind,
      name: chunk.name,
      summary: summary.trim(),
      exports: chunk.exports,
      imports: chunk.imports,
      rawCode: chunk.code,
    };
  }

  async analyzeFile(
    chunks: RawChunk[],
    filePath: string,
    language: string,
  ): Promise<CodeUnit[]> {
    return Promise.all(
      chunks.map(chunk => this.analyzeUnit(chunk, filePath, language)),
    );
  }
}
