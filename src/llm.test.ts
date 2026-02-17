import { describe, it, expect, vi } from 'vitest';
import { LlmClient } from './llm.js';

describe('LlmClient', () => {
  it('respects max concurrency', async () => {
    let activeCalls = 0;
    let maxActive = 0;

    const mockClient = {
      messages: {
        create: vi.fn(async () => {
          activeCalls++;
          maxActive = Math.max(maxActive, activeCalls);
          await new Promise(r => setTimeout(r, 50));
          activeCalls--;
          return {
            content: [{ type: 'text', text: '{"summary": "test"}' }],
          };
        }),
      },
    };

    const llm = new LlmClient(mockClient as any, { maxConcurrent: 2 });

    await Promise.all([
      llm.complete('prompt1', 'haiku'),
      llm.complete('prompt2', 'haiku'),
      llm.complete('prompt3', 'haiku'),
      llm.complete('prompt4', 'haiku'),
    ]);

    expect(maxActive).toBeLessThanOrEqual(2);
    expect(mockClient.messages.create).toHaveBeenCalledTimes(4);
  });

  it('uses correct model for each tier', async () => {
    const mockClient = {
      messages: {
        create: vi.fn(async () => ({
          content: [{ type: 'text', text: 'response' }],
        })),
      },
    };

    const llm = new LlmClient(mockClient as any, {
      haikuModel: 'test-haiku',
      opusModel: 'test-opus',
    });

    await llm.complete('prompt', 'haiku');
    expect(mockClient.messages.create).toHaveBeenCalledWith(
      expect.objectContaining({ model: 'test-haiku' }),
    );

    await llm.complete('prompt', 'opus');
    expect(mockClient.messages.create).toHaveBeenCalledWith(
      expect.objectContaining({ model: 'test-opus' }),
    );
  });

  it('parses JSON from code fences', async () => {
    const mockClient = {
      messages: {
        create: vi.fn(async () => ({
          content: [{ type: 'text', text: '```json\n{"key": "value"}\n```' }],
        })),
      },
    };

    const llm = new LlmClient(mockClient as any);
    const result = await llm.completeJson<{ key: string }>('prompt', 'haiku');
    expect(result.key).toBe('value');
  });
});
