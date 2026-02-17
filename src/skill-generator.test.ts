import { describe, it, expect } from 'vitest';
import { SkillGenerator } from './skill-generator.js';
import type { CodebasePatterns, SystemNarrative, Domain } from './types.js';

describe('SkillGenerator', () => {
  const patterns: CodebasePatterns = {
    naming: [
      { rule: 'camelCase for functions and variables', examples: ['validateToken', 'getUserById'], confidence: 'high' },
      { rule: 'PascalCase for classes and interfaces', examples: ['UserService', 'AuthMiddleware'], confidence: 'high' },
    ],
    fileOrganization: [
      { rule: 'Domain-based directory structure under src/', examples: ['src/auth/', 'src/users/'], confidence: 'high' },
    ],
    architecture: [
      { rule: 'All DB access through repository classes', examples: ['UserRepository.findById()'], confidence: 'medium' },
    ],
    imports: [
      { rule: 'Absolute imports from @/ prefix', examples: ['@/utils/validate'], confidence: 'high' },
    ],
    errorHandling: [
      { rule: 'API handlers wrap in try/catch with standardized error response', examples: [], confidence: 'medium' },
    ],
    testing: [
      { rule: 'Co-located test files with .test.ts suffix', examples: ['auth.test.ts'], confidence: 'high' },
    ],
    domainBoundaries: [
      { rule: 'Auth domain must not import directly from Payment domain', examples: [], confidence: 'medium' },
    ],
  };

  const system: SystemNarrative = {
    layer: 4,
    overview: 'A REST API for user management',
    architecture: 'Layered architecture with Express middleware',
    domainInteractions: 'Auth guards all routes',
    entryPoints: ['src/index.ts'],
    techStack: ['TypeScript', 'Express', 'PostgreSQL'],
    risks: [],
  };

  it('generates CLAUDE.md with pattern rules', () => {
    const generator = new SkillGenerator();
    const output = generator.generateClaudeMd(patterns, system, []);

    expect(output).toContain('camelCase for functions');
    expect(output).toContain('PascalCase for classes');
    expect(output).toContain('Domain-based directory');
    expect(output).toContain('repository classes');
    expect(output).toContain('# Codebase Intelligence');
  });

  it('generates .cursorrules', () => {
    const generator = new SkillGenerator();
    const output = generator.generateCursorRules(patterns, system, []);

    expect(output).toContain('camelCase');
    expect(output).toContain('Domain-based');
  });

  it('only includes high-confidence rules by default when minConfidence is high', () => {
    const generator = new SkillGenerator();
    const output = generator.generateClaudeMd(patterns, system, [], { minConfidence: 'high' });

    expect(output).toContain('camelCase');
    expect(output).not.toContain('repository classes'); // medium confidence
  });

  it('includes domain sections when domains are provided', () => {
    const generator = new SkillGenerator();
    const domains: Domain[] = [{
      layer: 3,
      domain: 'Authentication',
      description: 'Handles user login and token management.',
      units: ['auth.ts::validate'],
      entryPoints: ['auth.ts::validate'],
      dataFlow: 'Request -> validate -> respond',
      concerns: ['Token expiry not handled'],
    }];
    const output = generator.generateClaudeMd(patterns, system, domains);

    expect(output).toContain('### Authentication');
    expect(output).toContain('Handles user login');
    expect(output).toContain('Token expiry not handled');
  });
});
