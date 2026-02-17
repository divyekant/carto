import { describe, it, expect, vi } from 'vitest';
import { DeepAnalyzer } from './deep.js';
import type { CodeUnit } from '../types.js';

const MOCK_OPUS_RESPONSE = JSON.stringify({
  relationships: [
    {
      type: 'calls',
      from: 'src/auth/middleware.ts::validateToken',
      to: 'src/auth/jwt.ts::verifyJWT',
      description: 'Delegates JWT verification',
    },
  ],
  domains: [
    {
      domain: 'Authentication',
      description: 'Handles user authentication and token management',
      units: ['src/auth/middleware.ts::validateToken', 'src/auth/jwt.ts::verifyJWT'],
      entryPoints: ['src/auth/middleware.ts::validateToken'],
      dataFlow: 'Request -> validateToken -> verifyJWT',
      concerns: ['No token refresh mechanism'],
    },
  ],
  system: {
    overview: 'A web API with JWT authentication',
    architecture: 'Layered architecture with middleware-based auth',
    domainInteractions: 'Auth domain guards all API routes',
    entryPoints: ['src/index.ts'],
    techStack: ['TypeScript', 'Express', 'JWT'],
    risks: ['Single point of failure in auth middleware'],
  },
  patterns: {
    naming: [{ rule: 'camelCase for functions', examples: ['validateToken', 'verifyJWT'], confidence: 'high' }],
    fileOrganization: [{ rule: 'Domain-based directory structure', examples: ['src/auth/'], confidence: 'high' }],
    architecture: [],
    imports: [],
    errorHandling: [],
    testing: [],
    domainBoundaries: [],
  },
});

describe('DeepAnalyzer', () => {
  it('produces relationships, domains, system narrative, and patterns from units', async () => {
    const mockLlm = {
      complete: vi.fn(async () => MOCK_OPUS_RESPONSE),
    };

    const analyzer = new DeepAnalyzer(mockLlm as any);

    const units: CodeUnit[] = [
      {
        id: 'src/auth/middleware.ts::validateToken',
        layer: 1, path: 'src/auth/middleware.ts', lines: [1, 10] as [number, number],
        language: 'typescript', kind: 'function', name: 'validateToken',
        summary: 'Validates JWT tokens', exports: ['validateToken'],
        imports: ['./jwt'], rawCode: 'function validateToken() {}',
      },
      {
        id: 'src/auth/jwt.ts::verifyJWT',
        layer: 1, path: 'src/auth/jwt.ts', lines: [1, 8] as [number, number],
        language: 'typescript', kind: 'function', name: 'verifyJWT',
        summary: 'Verifies JWT signature', exports: ['verifyJWT'],
        imports: ['jsonwebtoken'], rawCode: 'function verifyJWT() {}',
      },
    ];

    const result = await analyzer.analyze(units);

    expect(result.relationships).toHaveLength(1);
    expect(result.relationships[0].type).toBe('calls');
    expect(result.relationships[0].layer).toBe(2);
    expect(result.domains).toHaveLength(1);
    expect(result.domains[0].domain).toBe('Authentication');
    expect(result.domains[0].layer).toBe(3);
    expect(result.system.overview).toContain('JWT');
    expect(result.system.layer).toBe(4);
    expect(result.patterns.naming).toHaveLength(1);
    expect(mockLlm.complete).toHaveBeenCalledWith(
      expect.any(String),
      'opus',
      expect.any(Object),
    );
  });

  it('handles snake_case keys in LLM response', async () => {
    const snakeCaseResponse = JSON.stringify({
      relationships: [],
      domains: [{
        domain: 'Test',
        description: 'Test domain',
        units: [],
        entry_points: ['main.ts::main'],
        data_flow: 'in -> out',
        concerns: [],
      }],
      system: {
        overview: 'A test app',
        architecture: 'Simple',
        domain_interactions: 'None',
        entry_points: ['main.ts'],
        tech_stack: ['TypeScript'],
        risks: [],
      },
      patterns: {
        naming: [],
        file_organization: [{ rule: 'flat', examples: [], confidence: 'low' }],
        architecture: [],
        imports: [],
        error_handling: [],
        testing: [],
        domain_boundaries: [],
      },
    });

    const mockLlm = { complete: vi.fn(async () => snakeCaseResponse) };
    const analyzer = new DeepAnalyzer(mockLlm as any);
    const result = await analyzer.analyze([]);

    expect(result.domains[0].entryPoints).toContain('main.ts::main');
    expect(result.domains[0].dataFlow).toBe('in -> out');
    expect(result.system.domainInteractions).toBe('None');
    expect(result.system.techStack).toContain('TypeScript');
    expect(result.patterns.fileOrganization).toHaveLength(1);
  });
});
