import { apiFetchRaw } from '@/lib/api'

type EventHandler = (data: string) => void

interface SSEMessage {
  event: string
  data: string
}

interface ConnectSSEOptions {
  onError?: (error: Error) => void
}

export interface SSEConnection {
  close: () => void
  done: Promise<void>
}

function parseEventBlock(block: string): SSEMessage | null {
  let event = 'message'
  const dataLines: string[] = []

  for (const rawLine of block.split(/\r?\n/)) {
    if (rawLine.startsWith(':') || rawLine.trim() === '') {
      continue
    }
    if (rawLine.startsWith('event:')) {
      event = rawLine.slice(6).trim()
      continue
    }
    if (rawLine.startsWith('data:')) {
      dataLines.push(rawLine.slice(5).trimStart())
    }
  }

  if (dataLines.length === 0) {
    return null
  }

  return { event, data: dataLines.join('\n') }
}

function findBlockSeparator(buffer: string): number {
  const unixIndex = buffer.indexOf('\n\n')
  const windowsIndex = buffer.indexOf('\r\n\r\n')

  if (unixIndex === -1) return windowsIndex
  if (windowsIndex === -1) return unixIndex
  return Math.min(unixIndex, windowsIndex)
}

export async function streamSSE(
  path: string,
  onMessage: (message: SSEMessage) => void,
  signal?: AbortSignal,
) {
  const response = await apiFetchRaw(path, {
    headers: { Accept: 'text/event-stream' },
    signal,
  })

  if (!response.body) {
    throw new Error('Progress stream is unavailable')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        buffer += decoder.decode()
        if (buffer.trim() !== '') {
          const parsed = parseEventBlock(buffer)
          if (parsed) {
            onMessage(parsed)
          }
        }
        return
      }

      buffer += decoder.decode(value, { stream: true })

      while (true) {
        const separatorIndex = findBlockSeparator(buffer)
        if (separatorIndex === -1) {
          break
        }

        const block = buffer.slice(0, separatorIndex)
        const separatorLength = buffer.startsWith('\r\n\r\n', separatorIndex) ? 4 : 2
        buffer = buffer.slice(separatorIndex + separatorLength)
        const parsed = parseEventBlock(block)
        if (parsed) {
          onMessage(parsed)
        }
      }
    }
  } finally {
    try {
      await reader.cancel()
    } catch {
      // Stream may already be closed by the server.
    }
  }
}

export function connectSSE(
  path: string,
  handlers: Record<string, EventHandler>,
  options: ConnectSSEOptions = {},
): SSEConnection {
  const controller = new AbortController()

  const done = streamSSE(path, ({ event, data }) => {
    handlers[event]?.(data)
  }, controller.signal).catch((error) => {
    if (controller.signal.aborted) {
      return
    }
    options.onError?.(error instanceof Error ? error : new Error(String(error)))
  })

  return {
    close: () => controller.abort(),
    done,
  }
}
