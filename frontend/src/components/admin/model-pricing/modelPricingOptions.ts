import type { ModelPricingItem } from '@/api/admin/modelPricing'

export type ModelPricingProvider = 'anthropic' | 'openai' | 'gemini' | 'antigravity'
export type ModelPricingBillingMode = 'token' | 'per_request' | 'image'

export const MODEL_PRICING_PROVIDER_OPTIONS: Array<{ value: ModelPricingProvider; label: string }> = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
]

const MODEL_PRICING_PROVIDERS = new Set<string>(
  MODEL_PRICING_PROVIDER_OPTIONS.map((option) => option.value)
)

export function normalizeModelPricingProvider(provider?: string | null): ModelPricingProvider | '' {
  const value = (provider || '').trim().toLowerCase()
  if (MODEL_PRICING_PROVIDERS.has(value)) {
    return value as ModelPricingProvider
  }
  if (value === 'text-completion-openai') return 'openai'
  if (value.startsWith('vertex_ai')) return 'gemini'
  return ''
}

export function inferModelPricingProvider(model?: string | null): ModelPricingProvider | '' {
  const value = (model || '').trim().toLowerCase()
  if (value.startsWith('claude-')) return 'anthropic'
  if (value.startsWith('gemini-')) return 'gemini'
  if (
    value.startsWith('gpt-') ||
    value.startsWith('o1') ||
    value.startsWith('o3') ||
    value.startsWith('o4') ||
    value.startsWith('chatgpt-')
  ) {
    return 'openai'
  }
  return ''
}

export function resolveModelPricingProvider(
  item?: Pick<ModelPricingItem, 'model' | 'provider' | 'global_override' | 'billing_basis_hint'> | null,
  fallback?: string | null
): ModelPricingProvider {
  return (
    normalizeModelPricingProvider(item?.global_override?.provider) ||
    normalizeModelPricingProvider(item?.billing_basis_hint?.platform) ||
    normalizeModelPricingProvider(fallback) ||
    normalizeModelPricingProvider(item?.provider) ||
    inferModelPricingProvider(item?.model) ||
    'antigravity'
  )
}

export function modelPricingProviderLabel(provider?: string | null): string {
  const normalized = normalizeModelPricingProvider(provider)
  return MODEL_PRICING_PROVIDER_OPTIONS.find((option) => option.value === normalized)?.label || ''
}

export function normalizeModelPricingBillingMode(mode?: string | null): ModelPricingBillingMode {
  switch ((mode || '').trim().toLowerCase()) {
    case 'per_request':
      return 'per_request'
    case 'image':
      return 'image'
    default:
      return 'token'
  }
}
