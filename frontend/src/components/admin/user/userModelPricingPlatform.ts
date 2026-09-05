import { perTokenToMTok } from '@/components/admin/channel/types'
import {
  inferModelPricingProvider,
  MODEL_PRICING_PROVIDER_OPTIONS,
  normalizeModelPricingProvider,
  type ModelPricingProvider,
} from '@/components/admin/model-pricing/modelPricingOptions'

export type UserPricingTab = ModelPricingProvider | 'other'

export type CatalogMap = Record<ModelPricingProvider, string[]>

export type BillingPriceField =
  | 'input_price'
  | 'output_price'
  | 'cache_write_price'
  | 'cache_write_1h_price'
  | 'cache_read_price'

export interface SuggestedPriceSource {
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
}

export interface PricingProviderHint {
  model?: string
  provider?: string | null
  global_override?: { provider?: string | null } | null
  billing_basis_hint?: { platform?: string | null } | null
}

export interface UserModelPricingFormRow {
  id?: number
  localKey: string
  model: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null
  display_cache_creation_price: number | null
  display_cache_creation_1h_price: number | null
  enabled: boolean
  notes: string
  addedOnTab?: UserPricingTab
}

export const USER_PRICING_PLATFORM_TABS: ModelPricingProvider[] = MODEL_PRICING_PROVIDER_OPTIONS.map(
  (option) => option.value
)

export function emptyCatalogMap(): CatalogMap {
  return {
    anthropic: [],
    openai: [],
    gemini: [],
    antigravity: [],
  }
}

export function emptyOverrideRow(
  model = '',
  addedOnTab?: UserPricingTab,
  localKey = `new-${Math.random().toString(36).slice(2, 10)}`
): UserModelPricingFormRow {
  return {
    localKey,
    model,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    display_input_price: null,
    display_output_price: null,
    display_cache_read_price: null,
    display_cache_creation_price: null,
    display_cache_creation_1h_price: null,
    enabled: true,
    notes: '',
    addedOnTab,
  }
}

export function resolvedProviderWithoutDefault(
  model: string,
  item?: PricingProviderHint | null
): ModelPricingProvider | '' {
  return (
    normalizeModelPricingProvider(item?.global_override?.provider) ||
    normalizeModelPricingProvider(item?.billing_basis_hint?.platform) ||
    normalizeModelPricingProvider(item?.provider) ||
    inferModelPricingProvider(model)
  )
}

export function platformsForModel(
  model: string,
  catalogs: CatalogMap,
  item?: PricingProviderHint | null
): ModelPricingProvider[] {
  const key = model.trim().toLowerCase()
  if (!key) return []

  const out: ModelPricingProvider[] = []
  const seen = new Set<ModelPricingProvider>()
  for (const platform of USER_PRICING_PLATFORM_TABS) {
    if (catalogs[platform].some((id) => id.trim().toLowerCase() === key)) {
      seen.add(platform)
      out.push(platform)
    }
  }
  const inferred = resolvedProviderWithoutDefault(model, item)
  if (inferred && !seen.has(inferred)) {
    out.push(inferred)
  }
  return out
}

export function rowVisibleOnTab(
  row: Pick<UserModelPricingFormRow, 'model' | 'addedOnTab'>,
  tab: UserPricingTab,
  catalogs: CatalogMap,
  item?: PricingProviderHint | null
): boolean {
  const model = row.model.trim()
  if (!model) {
    return (row.addedOnTab ?? 'other') === tab
  }
  const platforms = platformsForModel(model, catalogs, item)
  if (tab === 'other') return platforms.length === 0
  return platforms.includes(tab)
}

export function litellmSuggestedMTok(
  source: SuggestedPriceSource | undefined,
  field: BillingPriceField
): number | null {
  if (!source) return null
  const perToken = source[field]
  if (perToken == null) return null
  return perTokenToMTok(perToken)
}

export function applySuggestedBilling(
  item: UserModelPricingFormRow,
  suggestedMTok: (field: BillingPriceField) => number | null
): void {
  const fields: BillingPriceField[] = [
    'input_price',
    'output_price',
    'cache_write_price',
    'cache_write_1h_price',
    'cache_read_price',
  ]
  for (const field of fields) {
    const value = suggestedMTok(field)
    if (value != null) item[field] = value
  }
}

export function applySuggestedDisplay(
  item: UserModelPricingFormRow,
  suggestedMTok: (field: BillingPriceField) => number | null
): void {
  const inputMTok = suggestedMTok('input_price')
  if (inputMTok != null) item.display_input_price = inputMTok
  const outputMTok = suggestedMTok('output_price')
  if (outputMTok != null) item.display_output_price = outputMTok
  const cacheReadMTok = suggestedMTok('cache_read_price')
  if (cacheReadMTok != null) item.display_cache_read_price = cacheReadMTok
  const cacheWriteMTok = suggestedMTok('cache_write_price')
  if (cacheWriteMTok != null) item.display_cache_creation_price = cacheWriteMTok
  const cacheWrite1hMTok = suggestedMTok('cache_write_1h_price')
  if (cacheWrite1hMTok != null && cacheWrite1hMTok > 0) item.display_cache_creation_1h_price = cacheWrite1hMTok
}

export function applySuggestedBoth(
  item: UserModelPricingFormRow,
  suggestedMTok: (field: BillingPriceField) => number | null
): void {
  applySuggestedBilling(item, suggestedMTok)
  applySuggestedDisplay(item, suggestedMTok)
}

export function ensureCuratedOverrideRows(
  rows: UserModelPricingFormRow[],
  curatedIds: string[],
  addedOnTab: UserPricingTab
): UserModelPricingFormRow[] {
  const have = new Set(rows.map((row) => row.model.trim().toLowerCase()).filter(Boolean))
  const next = rows.slice()
  for (const id of curatedIds) {
    const model = id.trim()
    if (!model) continue
    const key = model.toLowerCase()
    if (have.has(key)) continue
    have.add(key)
    next.push(emptyOverrideRow(model, addedOnTab))
  }
  return next
}

export function applySuggestedToCurated(
  rows: UserModelPricingFormRow[],
  curatedIds: string[],
  suggestedMTokForModel: (model: string, field: BillingPriceField) => number | null
): void {
  const want = new Set(curatedIds.map((id) => id.trim().toLowerCase()).filter(Boolean))
  for (const row of rows) {
    if (!want.has(row.model.trim().toLowerCase())) continue
    applySuggestedBoth(row, (field) => suggestedMTokForModel(row.model, field))
  }
}

export function modelSelectOptions(args: {
  tab: UserPricingTab
  catalogs: CatalogMap
  available: Array<{ model: string; provider: string }>
  extraModels: string[]
}): Array<{ value: string; label: string }> {
  const seen = new Set<string>()
  const opts: Array<{ value: string; label: string }> = []

  const add = (model: string, provider = '') => {
    const value = model.trim()
    if (!value) return
    const key = value.toLowerCase()
    if (seen.has(key)) return
    seen.add(key)
    opts.push({
      value,
      label: provider ? `${value}  ·  ${provider}` : value,
    })
  }

  if (args.tab !== 'other') {
    for (const id of args.catalogs[args.tab]) {
      const info = args.available.find((item) => item.model.trim().toLowerCase() === id.trim().toLowerCase())
      add(id, info?.provider || args.tab)
    }
  }

  for (const item of args.available) {
    if (rowVisibleOnTab({ model: item.model }, args.tab, args.catalogs, item)) {
      add(item.model, item.provider)
    }
  }

  for (const model of args.extraModels) {
    add(model)
  }

  return opts
}
