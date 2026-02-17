import { describe, it, expect, vi, beforeEach } from 'vitest';
import { loadConfig } from './config.js';
import { FaissClient } from './faiss-client.js';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

describe('loadConfig', () => {
  it('loads config from environment variables', () => {
    const config = loadConfig();
    expect(config.faissUrl).toBeDefined();
    expect(config.anthropicApiKey).toBeDefined();
    expect(config.haikuModel).toBeDefined();
    expect(config.opusModel).toBeDefined();
  });
});

describe('FaissClient', () => {
  let client: FaissClient;

  beforeEach(() => {
    client = new FaissClient('http://localhost:8900', 'test-key');
    mockFetch.mockReset();
  });

  it('adds a memory with correct headers and body', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, id: 42 }),
    });

    const id = await client.addMemory({
      text: 'test memory',
      source: 'test/source',
      metadata: { layer: 1 },
    });

    expect(id).toBe(42);
    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8900/memory/add',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'X-API-Key': 'test-key',
        }),
      }),
    );
  });

  it('searches with hybrid mode', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        results: [{ id: 1, text: 'result', score: 0.9, source: 'test' }],
        count: 1,
      }),
    });

    const results = await client.search('auth token', { k: 5 });
    expect(results).toHaveLength(1);
    expect(results[0].score).toBe(0.9);
  });

  it('batch adds memories', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, added: 3 }),
    });

    await client.addBatch([
      { text: 'mem1', source: 's1' },
      { text: 'mem2', source: 's2' },
      { text: 'mem3', source: 's3' },
    ]);

    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.memories).toHaveLength(3);
  });

  it('deletes memories by source prefix', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        memories: [
          { id: 1, text: 't1', source: 'codex/proj/layer:0' },
          { id: 2, text: 't2', source: 'codex/proj/layer:0' },
        ],
        total: 2,
      }),
    });
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({ success: true }) });
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({ success: true }) });

    const count = await client.deleteBySource('codex/proj/layer:0');
    expect(count).toBe(2);
  });
});
