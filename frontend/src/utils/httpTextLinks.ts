export interface HttpTextPart {
  text: string
  href?: string
}

const HTTP_URL_PATTERN = /https?:\/\/[^\s<>"']+/gi
const TRAILING_URL_PUNCTUATION = /[),.;:!?，。、；：！？）\]]+$/u

function trimUrlTail(raw: string): { href: string; tail: string } {
  const href = raw.replace(TRAILING_URL_PUNCTUATION, '')
  return { href, tail: raw.slice(href.length) }
}

/** Accept only absolute http(s) URLs with no embedded credentials. */
export function isSafeHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return false
    if (!parsed.hostname) return false
    if (parsed.username || parsed.password) return false
    return true
  } catch {
    return false
  }
}

export function extractFirstHttpUrl(text: string): string | null {
  if (!text) return null
  const match = text.match(HTTP_URL_PATTERN)
  if (!match) return null
  for (const raw of match) {
    const { href } = trimUrlTail(raw)
    if (isSafeHttpUrl(href)) return href
  }
  return null
}

export function linkifyHttpText(text: string): HttpTextPart[] {
  if (!text) return []
  const parts: HttpTextPart[] = []
  const pattern = new RegExp(HTTP_URL_PATTERN.source, 'gi')
  let last = 0
  for (const match of text.matchAll(pattern)) {
    const raw = match[0]
    const index = match.index ?? 0
    const { href, tail } = trimUrlTail(raw)
    if (index > last) parts.push({ text: text.slice(last, index) })
    if (isSafeHttpUrl(href)) {
      parts.push({ text: href, href })
      if (tail) parts.push({ text: tail })
    } else {
      parts.push({ text: raw })
    }
    last = index + raw.length
  }
  if (last < text.length) parts.push({ text: text.slice(last) })
  if (parts.length === 0) parts.push({ text })
  return parts
}
