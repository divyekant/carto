import Anthropic from '@anthropic-ai/sdk';

interface LlmOptions {
  maxConcurrent?: number;
  haikuModel?: string;
  opusModel?: string;
}

type ModelTier = 'haiku' | 'opus';

export class LlmClient {
  private semaphore: number;
  private queue: Array<() => void> = [];
  private active = 0;
  private haikuModel: string;
  private opusModel: string;

  constructor(
    private client: Anthropic,
    options: LlmOptions = {},
  ) {
    this.semaphore = options.maxConcurrent ?? 5;
    this.haikuModel = options.haikuModel ?? 'claude-haiku-4-5-20251001';
    this.opusModel = options.opusModel ?? 'claude-opus-4-6';
  }

  async complete(
    prompt: string,
    tier: ModelTier,
    options?: { system?: string; maxTokens?: number },
  ): Promise<string> {
    await this.acquire();
    try {
      const model = tier === 'haiku' ? this.haikuModel : this.opusModel;
      const response = await this.client.messages.create({
        model,
        max_tokens: options?.maxTokens ?? 4096,
        system: options?.system,
        messages: [{ role: 'user', content: prompt }],
      });

      const textBlock = response.content.find((b: any) => b.type === 'text');
      return (textBlock as any)?.text ?? '';
    } finally {
      this.release();
    }
  }

  async completeJson<T>(
    prompt: string,
    tier: ModelTier,
    options?: { system?: string; maxTokens?: number },
  ): Promise<T> {
    const text = await this.complete(prompt, tier, options);
    const jsonMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/);
    const jsonStr = jsonMatch ? jsonMatch[1].trim() : text.trim();
    return JSON.parse(jsonStr);
  }

  private acquire(): Promise<void> {
    if (this.active < this.semaphore) {
      this.active++;
      return Promise.resolve();
    }
    return new Promise<void>(resolve => {
      this.queue.push(() => {
        this.active++;
        resolve();
      });
    });
  }

  private release(): void {
    this.active--;
    const next = this.queue.shift();
    if (next) next();
  }
}
