import source from './messages.zh-Hans.json'

type MessageKey = keyof typeof source
type Messages = Partial<Record<MessageKey, string>>

function injected(): Messages {
  const element = document.querySelector<HTMLScriptElement>('script[data-ui-messages]')
  if (!element?.textContent) return {}
  try {
    const value: unknown = JSON.parse(element.textContent)
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return {}
    const result: Messages = {}
    for (const key of Object.keys(source) as MessageKey[]) {
      const candidate = (value as Record<string, unknown>)[key]
      if (typeof candidate === 'string' && candidate.length <= 500) result[key] = candidate
    }
    return result
  } catch { return {} }
}

const messages = typeof document === 'undefined' ? {} : injected()

export function t(key: MessageKey): string {
  return messages[key] ?? source[key]
}
