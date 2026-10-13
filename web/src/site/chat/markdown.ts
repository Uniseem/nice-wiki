export interface AnswerPart { type: 'text'|'strong'|'code'|'citation'|'break', text: string }

export function parseAnswer(value: string): AnswerPart[] {
  const result: AnswerPart[] = []
  const pattern = /(`[^`\n]+`|\*\*[^*\n]+\*\*|\[块\s+[A-Za-z0-9_-]+\]|\n)/g
  let start = 0
  for (const match of value.matchAll(pattern)) {
    const index = match.index
    if (index > start) result.push({ type: 'text', text: value.slice(start, index) })
    const token = match[0]
    if (token === '\n') result.push({ type: 'break', text: '' })
    else if (token.startsWith('`')) result.push({ type: 'code', text: token.slice(1, -1) })
    else if (token.startsWith('**')) result.push({ type: 'strong', text: token.slice(2, -2) })
    else result.push({ type: 'citation', text: token })
    start = index + token.length
  }
  if (start < value.length) result.push({ type: 'text', text: value.slice(start) })
  return result
}
