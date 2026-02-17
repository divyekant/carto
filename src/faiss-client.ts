export interface SearchResult {
  id: number;
  text: string;
  score: number;
  source: string;
  metadata?: Record<string, unknown>;
}

export interface SearchOptions {
  k?: number;
  threshold?: number;
  hybrid?: boolean;
  source?: string;
}

export class FaissClient {
  constructor(
    private baseUrl: string,
    private apiKey: string,
  ) {}

  private async request(path: string, options: RequestInit = {}): Promise<Response> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': this.apiKey,
        ...options.headers,
      },
    });
    if (!res.ok) {
      const text = await res.text().catch(() => 'unknown error');
      throw new Error(`FAISS API error ${res.status}: ${text}`);
    }
    return res;
  }

  async addMemory(memory: {
    text: string;
    source: string;
    metadata?: Record<string, unknown>;
    deduplicate?: boolean;
  }): Promise<number> {
    const res = await this.request('/memory/add', {
      method: 'POST',
      body: JSON.stringify({
        text: memory.text,
        source: memory.source,
        metadata: memory.metadata,
        deduplicate: memory.deduplicate ?? false,
      }),
    });
    const data = await res.json();
    return data.id;
  }

  async addBatch(
    memories: { text: string; source: string; metadata?: Record<string, unknown> }[],
    deduplicate = false,
  ): Promise<void> {
    for (let i = 0; i < memories.length; i += 500) {
      const batch = memories.slice(i, i + 500);
      await this.request('/memory/add-batch', {
        method: 'POST',
        body: JSON.stringify({ memories: batch, deduplicate }),
      });
    }
  }

  async search(query: string, options: SearchOptions = {}): Promise<SearchResult[]> {
    const res = await this.request('/search', {
      method: 'POST',
      body: JSON.stringify({
        query,
        k: options.k ?? 10,
        threshold: options.threshold,
        hybrid: options.hybrid ?? true,
      }),
    });
    const data = await res.json();
    return data.results;
  }

  async listBySource(source: string, limit = 50): Promise<SearchResult[]> {
    const res = await this.request(`/memories?source=${encodeURIComponent(source)}&limit=${limit}`);
    const data = await res.json();
    return data.memories;
  }

  async deleteMemory(id: number): Promise<void> {
    await this.request(`/memory/${id}`, { method: 'DELETE' });
  }

  async deleteBySource(sourcePrefix: string): Promise<number> {
    const memories = await this.listBySource(sourcePrefix);
    for (const mem of memories) {
      await this.deleteMemory(mem.id);
    }
    return memories.length;
  }

  async health(): Promise<boolean> {
    try {
      const res = await this.request('/health');
      return res.ok;
    } catch {
      return false;
    }
  }
}
