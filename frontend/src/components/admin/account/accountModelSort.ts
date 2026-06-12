import type { ClaudeModel } from '@/types'

const preferredModelPatterns = [
  /claude.*opus.*4[.-]?8/,
  /opus.*4[.-]?8/,
  /gpt.*5[.-]?5/,
  /gpt.*5[.-]?4/,
  /gpt.*5[.-]?2/,
  /gpt.*5/,
  /claude.*sonnet.*4[.-]?5/,
  /sonnet.*4[.-]?5/,
  /claude.*opus.*4/,
  /opus.*4/,
  /claude.*sonnet.*4/,
  /sonnet.*4/,
  /gemini.*3[.-]?1.*image/,
  /gemini.*3[.-]?1/,
  /gemini.*3.*pro/,
  /gemini.*3.*flash/,
  /gemini.*2[.-]?5.*image/,
  /gemini.*2[.-]?5.*pro/,
  /gemini.*2[.-]?5.*flash/,
  /gpt.*4[.-]?1/,
  /gpt.*4o/,
  /claude.*haiku.*4[.-]?5/,
  /haiku.*4[.-]?5/,
  /gemini.*2[.-]?0/
]

function normalizeModelID(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '')
}

function modelSortText(model: ClaudeModel): string {
  return `${model.id} ${model.display_name || ''}`
}

function preferredModelRank(model: ClaudeModel): number {
  const text = modelSortText(model).toLowerCase()
  const compact = normalizeModelID(text)

  const rank = preferredModelPatterns.findIndex((pattern) => {
    return pattern.test(text) || pattern.test(compact)
  })

  return rank === -1 ? Number.MAX_SAFE_INTEGER : rank
}

function newestNumericTuple(model: ClaudeModel): number[] {
  const matches = modelSortText(model).match(/\d+(?:[.-]\d+)*/g) ?? []
  const best = matches
    .map((match) => match.split(/[.-]/).map((part) => Number.parseInt(part, 10)))
    .filter((parts) => parts.every(Number.isFinite))
    .sort((a, b) => compareNumberTuples(b, a))[0]

  return best ?? []
}

function compareNumberTuples(a: number[], b: number[]): number {
  const length = Math.max(a.length, b.length)
  for (let index = 0; index < length; index += 1) {
    const diff = (a[index] ?? 0) - (b[index] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}

export function sortAccountTestModels(models: ClaudeModel[]): ClaudeModel[] {
  return [...models].sort((a, b) => {
    const priorityDiff = preferredModelRank(a) - preferredModelRank(b)
    if (priorityDiff !== 0) return priorityDiff

    const versionDiff = compareNumberTuples(newestNumericTuple(b), newestNumericTuple(a))
    if (versionDiff !== 0) return versionDiff

    return a.id.localeCompare(b.id)
  })
}
