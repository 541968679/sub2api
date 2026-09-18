import type { PricingPageModel, PricingPagePlatform } from '@/api/pricingPage'

/** Synthetic tab id for domestic coding models split out of OpenAI (and similar) groups. */
export const DOMESTIC_PRICING_TAB = 'domestic'

const WESTERN_PREFIXES = [
  'gpt-',
  'chatgpt-',
  'claude-',
  'gemini-',
  'grok-',
  'dall-e',
  'gpt-image',
  'text-embedding',
  'text-moderation',
  'whisper-',
  'tts-',
  'sora-',
  'computer-use'
]

const DOMESTIC_PREFIXES = [
  'glm-',
  'chatglm',
  'kimi-',
  'moonshot-',
  'deepseek-',
  'minimax-',
  'abab',
  'qwen',
  'qwq-',
  'qwq',
  'mimo-',
  'hunyuan-',
  'hy3',
  'hy4',
  'doubao-',
  'ernie-',
  'spark-',
  'yi-',
  'baichuan',
  'internlm',
  'step-',
  'codegeex',
  'wenxin'
]

function startsWithAny(value: string, prefixes: string[]): boolean {
  return prefixes.some((prefix) => value === prefix || value.startsWith(prefix))
}

/** True for GLM / Kimi / DeepSeek / Qwen / MiniMax and other CN coding IDs. */
export function isDomesticPricingModel(model?: string | null): boolean {
  const value = (model || '').trim().toLowerCase()
  if (!value) return false
  if (startsWithAny(value, WESTERN_PREFIXES)) return false
  if (/^o[134]($|[-.]|\d)/.test(value)) return false
  return startsWithAny(value, DOMESTIC_PREFIXES)
}

/**
 * Pull domestic coding models into a sibling tab next to OpenAI so GPT and
 * 国产 IDs are not listed together. Empty source tabs are dropped.
 */
export function splitDomesticPricingPlatforms(
  platforms: PricingPagePlatform[]
): PricingPagePlatform[] {
  const result: PricingPagePlatform[] = []
  const domesticModels: PricingPageModel[] = []
  const seenDomestic = new Set<string>()
  let insertAt = -1

  for (const platform of platforms) {
    const kept: PricingPageModel[] = []
    let extracted = false
    for (const model of platform.models) {
      if (isDomesticPricingModel(model.model)) {
        extracted = true
        const key = model.model.trim().toLowerCase()
        if (!seenDomestic.has(key)) {
          seenDomestic.add(key)
          domesticModels.push(model)
        }
        continue
      }
      kept.push(model)
    }
    if (kept.length > 0) {
      if (platform.provider === 'openai') {
        insertAt = result.length + 1
      }
      result.push({ provider: platform.provider, models: kept })
      continue
    }
    if (extracted && insertAt < 0) {
      insertAt = result.length
    }
  }

  if (domesticModels.length === 0) return result
  if (insertAt < 0) insertAt = result.length
  insertAt = Math.min(insertAt, result.length)
  result.splice(insertAt, 0, {
    provider: DOMESTIC_PRICING_TAB,
    models: domesticModels
  })
  return result
}

function collectScaledPrices(usd: number | null | undefined, scale: number, cnyRate: number, out: number[]): void {
  if (usd == null || !Number.isFinite(usd)) return
  const scaled = usd * scale
  out.push(scaled)
  if (cnyRate > 0) out.push(scaled * cnyRate)
}

export function pricingModelPriceValues(model: PricingPageModel, cnyRate = 0): number[] {
  const prices: number[] = []
  collectScaledPrices(model.display_input_price, 1_000_000, cnyRate, prices)
  collectScaledPrices(model.display_output_price, 1_000_000, cnyRate, prices)
  collectScaledPrices(model.display_cache_read_price, 1_000_000, cnyRate, prices)
  collectScaledPrices(model.per_request_price, 1, cnyRate, prices)
  return prices
}

function parsePriceQueryToken(token: string): number | null {
  const raw = token.replace(/^[¥$]/, '')
  if (!/^\d+(\.\d+)?$/.test(raw)) return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}

function pricesMatchQuery(price: number, query: number): boolean {
  if (Math.abs(price - query) < 0.005) return true
  return price.toFixed(2) === query.toFixed(2)
}

export function normalizePricingSearchQuery(query?: string | null): string {
  return (query || '')
    .trim()
    .toLowerCase()
    .replace(/[，,]/g, ' ')
    .replace(/\s+/g, ' ')
}

/** Match model id, billing mode, and displayed USD/CNY unit prices. */
export function matchesPricingModelSearch(
  model: PricingPageModel,
  query?: string | null,
  cnyRate = 0
): boolean {
  const normalized = normalizePricingSearchQuery(query)
  if (!normalized) return true
  const name = `${model.model} ${model.billing_mode}`.toLowerCase()
  const prices = pricingModelPriceValues(model, cnyRate)
  return normalized.split(' ').every((token) => {
    if (name.includes(token)) return true
    const priceQuery = parsePriceQueryToken(token)
    if (priceQuery == null) return false
    return prices.some((price) => pricesMatchQuery(price, priceQuery))
  })
}

export function filterPricingModels(
  models: PricingPageModel[],
  query?: string | null,
  cnyRate = 0
): PricingPageModel[] {
  const normalized = normalizePricingSearchQuery(query)
  if (!normalized) return models
  return models.filter((model) => matchesPricingModelSearch(model, normalized, cnyRate))
}
