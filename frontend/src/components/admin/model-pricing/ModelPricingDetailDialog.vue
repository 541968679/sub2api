<template>
  <BaseDialog :show="show" @close="$emit('close')" :title="model" width="wide">
    <div v-if="loading" class="flex items-center justify-center py-12">
      <span class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
    </div>

    <div v-else-if="detail" class="space-y-6">
      <!-- LiteLLM Default Prices -->
      <section>
        <h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
          LiteLLM {{ t('admin.modelPricing.defaultPrices') }}
        </h3>
        <div v-if="detail.litellm_prices" class="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div v-for="field in litellmFields" :key="field.key" class="rounded bg-gray-50 px-3 py-2 dark:bg-gray-700/50">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ field.label }}</p>
            <p class="mt-0.5 font-mono text-sm text-gray-900 dark:text-white">
              {{ toMTok((detail.litellm_prices as Record<string, number>)[field.key]) }}
            </p>
          </div>
        </div>
        <p v-else class="text-sm text-gray-400">{{ t('admin.modelPricing.noLitellmData') }}</p>
      </section>

      <!-- Global Override -->
      <section class="rounded-lg border border-gray-200 p-4 dark:border-gray-600">
        <div class="mb-3 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300">
            {{ t('admin.modelPricing.globalOverride') }}
          </h3>
          <div class="flex items-center gap-4">
            <label class="flex items-center gap-2 text-sm" :title="t('admin.modelPricing.showOnPricingPageHint')">
              <input type="checkbox" v-model="form.show_on_pricing_page" class="rounded" />
              {{ t('admin.modelPricing.showOnPricingPage') }}
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input type="checkbox" v-model="form.enabled" class="rounded" />
              {{ t('admin.modelPricing.enabled') }}
            </label>
          </div>
        </div>

        <!-- Suggested prices hint (only for stub models without litellm + without existing override) -->
        <div
          v-if="detail.suggested_prices && !detail.global_override"
          class="mb-3 flex items-start justify-between gap-2 rounded-md bg-amber-50 px-3 py-2 text-xs dark:bg-amber-900/20"
        >
          <div class="min-w-0 flex-1 text-amber-800 dark:text-amber-300">
            <p class="font-semibold">
              💡 {{ t('admin.modelPricing.suggestedPricesHint', { from: detail.suggested_from || '' }) }}
            </p>
            <p class="mt-0.5 font-mono text-[11px] text-amber-700 dark:text-amber-400">
              <span>In: {{ toMTok(detail.suggested_prices.input_price) }}/MTok</span>
              <span class="ml-2">Out: {{ toMTok(detail.suggested_prices.output_price) }}/MTok</span>
              <span v-if="detail.suggested_prices.cache_write_price > 0" class="ml-2">CacheW: {{ toMTok(detail.suggested_prices.cache_write_price) }}</span>
              <span v-if="detail.suggested_prices.cache_read_price > 0" class="ml-2">CacheR: {{ toMTok(detail.suggested_prices.cache_read_price) }}</span>
            </p>
          </div>
          <button
            @click="applySuggested"
            class="btn btn-secondary shrink-0 text-xs"
            :disabled="saving"
          >
            {{ t('admin.modelPricing.applySuggested') }}
          </button>
        </div>

        <div class="mb-3 grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.billingModeLabel') }}</label>
            <Select v-model="form.billing_mode" :options="billingModeOptions" class="w-full" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500">Provider</label>
            <input v-model="form.provider" type="text" class="input text-sm w-full" :placeholder="detail.provider || ''" />
          </div>
        </div>

        <!-- Token mode fields -->
        <div v-if="form.billing_mode === 'token'" class="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.inputPrice') }} ($/MTok)</label>
            <input v-model="form.input_price" type="number" step="any" class="input text-sm w-full" :placeholder="litellmMTok('input_price')" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.outputPrice') }} ($/MTok)</label>
            <input v-model="form.output_price" type="number" step="any" class="input text-sm w-full" :placeholder="litellmMTok('output_price')" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.cacheWritePrice') }} ($/MTok)</label>
            <input v-model="form.cache_write_price" type="number" step="any" class="input text-sm w-full" :placeholder="litellmMTok('cache_write_price')" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.cacheReadPrice') }} ($/MTok)</label>
            <input v-model="form.cache_read_price" type="number" step="any" class="input text-sm w-full" :placeholder="litellmMTok('cache_read_price')" />
          </div>
          <div>
            <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.imageOutputPrice') }} ($/MTok)</label>
            <input v-model="form.image_output_price" type="number" step="any" class="input text-sm w-full" :placeholder="litellmMTok('image_output_price')" />
          </div>
        </div>

        <!-- Per-request mode fields -->
        <div v-else-if="form.billing_mode === 'per_request'" class="w-48">
          <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.perRequestPrice') }} ($)</label>
          <input v-model="form.per_request_price" type="number" step="any" min="0" class="input text-sm w-full" />
        </div>

        <!-- Image mode fields -->
        <div v-else-if="form.billing_mode === 'image'" class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.imageBillingStrategy') }}</label>
              <Select v-model="form.image_billing_strategy" :options="imageBillingStrategyOptions" class="w-full" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.imageFallbackPrice') }} ($)</label>
              <input v-model="form.per_request_price" type="number" step="any" min="0" class="input text-sm w-full" :placeholder="t('admin.modelPricing.imageFallbackPlaceholder')" />
            </div>
          </div>

          <div v-if="form.image_billing_strategy === 'megapixel'" class="space-y-3">
            <div class="w-full sm:w-48">
              <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.imageMegapixelDefault') }} ($/MP)</label>
              <input v-model="form.image_megapixel_price" type="number" step="any" min="0" class="input text-sm w-full" placeholder="0.3178914388" />
            </div>
            <div class="rounded-md border border-gray-200 p-3 dark:border-gray-600">
              <p class="mb-1 text-xs font-semibold text-gray-700 dark:text-gray-300">
                {{ t('admin.modelPricing.imageQualityPricesTitle') }}
              </p>
              <p class="mb-3 text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('admin.modelPricing.imageQualityPricesHint') }}
              </p>
              <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div v-for="quality in imageQualities" :key="quality">
                  <label class="mb-1 block text-xs text-gray-500">{{ quality }} ($/MP)</label>
                  <input v-model="form.image_quality_prices[quality]" type="number" step="any" min="0" class="input text-sm w-full" :placeholder="t('admin.modelPricing.imageQualityDefault')" />
                </div>
              </div>
            </div>
          </div>

          <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <div v-for="rule in form.image_tier_rules" :key="rule.tier_label" class="space-y-2">
              <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.modelPricing.imageTierLabel', { tier: rule.tier_label }) }}</label>
              <input v-model="rule.price" type="number" step="any" min="0" class="input text-sm w-full" :placeholder="t('admin.modelPricing.imageTierPricePlaceholder', { tier: rule.tier_label })" />
              <input v-model="rule.max_pixels" type="number" step="1" min="1" class="input text-sm w-full" :placeholder="t('admin.modelPricing.imageTierMaxPixelsPlaceholder')" />
            </div>
            <div class="col-span-2 rounded-md border border-gray-200 p-3 dark:border-gray-600 sm:col-span-3">
              <p class="mb-1 text-xs font-semibold text-gray-700 dark:text-gray-300">
                {{ t('admin.modelPricing.imageQualityMultipliersTitle') }}
              </p>
              <p class="mb-3 text-[11px] text-gray-500 dark:text-gray-400">
                {{ t('admin.modelPricing.imageQualityMultipliersHint') }}
              </p>
              <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <div v-for="quality in imageQualities" :key="quality">
                  <label class="mb-1 block text-xs text-gray-500">{{ quality }}</label>
                  <input v-model="form.image_quality_multipliers[quality]" type="number" step="any" min="0" class="input text-sm w-full" :placeholder="quality === 'auto' ? '1' : t('admin.modelPricing.imageQualityMultiplierDefault')" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Display Pricing Overrides (hidden for image mode — billed per image, display = actual) -->
        <div v-if="form.billing_mode !== 'image'" class="mt-4 rounded-lg border border-amber-200 bg-amber-50/50 p-3 dark:border-amber-800/30 dark:bg-amber-900/10">
          <div class="mb-2 flex items-center justify-between">
            <p class="text-xs font-semibold text-amber-700 dark:text-amber-400">{{ t('admin.modelPricing.displayPricingTitle') }}</p>
            <button
              v-if="hasDisplaySuggestion"
              @click="applyDisplaySuggested"
              class="btn btn-secondary shrink-0 text-xs"
              :disabled="saving"
            >
              {{ t('admin.modelPricing.applySuggested') }}
            </button>
          </div>
          <p class="mb-3 text-[10px] text-amber-600/70 dark:text-amber-500/60">{{ t('admin.modelPricing.displayPricingHint') }}</p>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div>
              <label class="mb-1 block text-[10px] text-gray-500">{{ t('admin.modelPricing.displayInputPrice') }}</label>
              <input v-model="form.display_input_price" type="number" step="any" min="0" class="input text-sm w-full" placeholder="--" />
            </div>
            <div>
              <label class="mb-1 block text-[10px] text-gray-500">{{ t('admin.modelPricing.displayOutputPrice') }}</label>
              <input v-model="form.display_output_price" type="number" step="any" min="0" class="input text-sm w-full" placeholder="--" />
            </div>
            <div>
              <label class="mb-1 block text-[10px] text-gray-500">{{ t('admin.modelPricing.displayCacheReadPrice') }}</label>
              <input v-model="form.display_cache_read_price" type="number" step="any" min="0" class="input text-sm w-full" placeholder="--" />
            </div>
            <div>
              <label class="mb-1 block text-[10px] text-gray-500" :title="t('admin.modelPricing.displayCacheCreationPriceHint')">{{ t('admin.modelPricing.displayCacheCreationPrice') }}</label>
              <input v-model="form.display_cache_creation_price" type="number" step="any" min="0" class="input text-sm w-full" placeholder="--" />
            </div>
          </div>
        </div>

        <div class="mt-3">
          <label class="mb-1 block text-xs text-gray-500">{{ t('admin.modelPricing.notes') }}</label>
          <input v-model="form.notes" type="text" class="input text-sm w-full" />
        </div>

        <div class="mt-4 flex justify-end gap-2">
          <button
            v-if="detail.global_override"
            @click="handleDelete"
            class="btn btn-secondary text-sm text-red-600 hover:text-red-700"
            :disabled="saving"
          >
            {{ t('common.delete') }}
          </button>
          <button @click="handleSave" class="btn btn-primary text-sm" :disabled="saving">
            <span v-if="saving" class="mr-1.5 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            {{ t('common.save') }}
          </button>
        </div>
      </section>

      <!-- Channel Overrides -->
      <section v-if="detail.channel_overrides && detail.channel_overrides.length > 0">
        <h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-300">
          {{ t('admin.modelPricing.channelOverrides') }} ({{ detail.channel_overrides.length }})
        </h3>
        <div class="space-y-1">
          <div
            v-for="ch in detail.channel_overrides"
            :key="ch.channel_id"
            class="flex items-center justify-between rounded bg-gray-50 px-3 py-2 text-sm dark:bg-gray-700/50"
          >
            <div>
              <span class="font-medium text-gray-900 dark:text-white">{{ ch.channel_name }}</span>
              <span class="ml-2 text-xs text-gray-400">{{ ch.platform }} / {{ ch.billing_mode }}</span>
            </div>
            <div class="flex gap-3 text-xs text-gray-500">
              <span v-if="ch.input_price != null">In: ${{ toMTok(ch.input_price) }}/MTok</span>
              <span v-if="ch.output_price != null">Out: ${{ toMTok(ch.output_price) }}/MTok</span>
              <router-link :to="`/admin/channels`" class="text-primary-600 hover:underline dark:text-primary-400">
                {{ t('admin.modelPricing.edit') }}
              </router-link>
            </div>
          </div>
        </div>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { ModelPricingDetail } from '@/api/admin/modelPricing'
import { perTokenToMTok, mTokToPerToken } from '@/components/admin/channel/types'
import { BaseDialog } from '@/components/common'
import Select from '@/components/common/Select.vue'
import type { ImageTierRule } from '@/api/admin/modelPricing'

const props = defineProps<{
  show: boolean
  model: string
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const detail = ref<ModelPricingDetail | null>(null)
const imageQualities = ['low', 'medium', 'high', 'auto'] as const

type ImageQuality = typeof imageQualities[number]
type ImageTierRuleForm = {
  tier_label: string
  max_pixels: string | number
  price: string | number
}

const form = reactive({
  enabled: true,
  billing_mode: 'token' as string,
  input_price: '' as string | number,
  output_price: '' as string | number,
  cache_write_price: '' as string | number,
  cache_read_price: '' as string | number,
  image_output_price: '' as string | number,
  per_request_price: '' as string | number,
  image_price_1k: '' as string | number,
  image_price_2k: '' as string | number,
  image_price_4k: '' as string | number,
  image_billing_strategy: 'tier' as 'tier' | 'megapixel',
  image_megapixel_price: '' as string | number,
  image_quality_prices: {
    low: '',
    medium: '',
    high: '',
    auto: '',
  } as Record<ImageQuality, string | number>,
  image_quality_multipliers: {
    low: '',
    medium: '',
    high: '',
    auto: 1,
  } as Record<ImageQuality, string | number>,
  image_tier_rules: [] as ImageTierRuleForm[],
  provider: '',
  notes: '',
  display_input_price: '' as string | number,
  display_output_price: '' as string | number,
  display_cache_read_price: '' as string | number,
  display_cache_creation_price: '' as string | number,
  show_on_pricing_page: false,
})

const billingModeOptions = computed(() => [
  { value: 'token', label: t('admin.modelPricing.billingModeToken') },
  { value: 'per_request', label: t('admin.modelPricing.billingModePerRequest') },
  { value: 'image', label: t('admin.modelPricing.billingModeImage') },
])

const imageBillingStrategyOptions = computed(() => [
  { value: 'megapixel', label: t('admin.modelPricing.imageBillingMegapixel') },
  { value: 'tier', label: t('admin.modelPricing.imageBillingTier') },
])

const defaultTierRules = (): ImageTierRuleForm[] => [
  { tier_label: '1K', max_pixels: 1048576, price: '' },
  { tier_label: '2K', max_pixels: 2359296, price: '' },
  { tier_label: '4K', max_pixels: '', price: '' },
]

const litellmFields = computed(() => [
  { key: 'input_price', label: t('admin.modelPricing.inputPrice') + ' ($/MTok)' },
  { key: 'output_price', label: t('admin.modelPricing.outputPrice') + ' ($/MTok)' },
  { key: 'cache_write_price', label: t('admin.modelPricing.cacheWritePrice') + ' ($/MTok)' },
  { key: 'cache_read_price', label: t('admin.modelPricing.cacheReadPrice') + ' ($/MTok)' },
  { key: 'image_output_price', label: t('admin.modelPricing.imageOutputPrice') + ' ($/MTok)' },
])

function toMTok(perToken: number | null | undefined): string {
  const v = perTokenToMTok(perToken)
  return v !== null ? `$${v}` : '-'
}

function litellmMTok(field: string): string {
  if (!detail.value?.litellm_prices) return ''
  const v = (detail.value.litellm_prices as Record<string, number>)[field]
  return v ? String(perTokenToMTok(v) ?? '') : ''
}

function resetImageQualityPrices(prices: Record<string, number> | null) {
  for (const quality of imageQualities) {
    form.image_quality_prices[quality] = prices?.[quality] ?? ''
  }
}

function resetImageQualityMultipliers(multipliers: Record<string, number> | null) {
  for (const quality of imageQualities) {
    form.image_quality_multipliers[quality] = multipliers?.[quality] ?? (quality === 'auto' ? 1 : '')
  }
}

function buildTierRuleForm(rules: ImageTierRule[] | null | undefined, go: NonNullable<ModelPricingDetail['global_override']>): ImageTierRuleForm[] {
  const source = rules && rules.length > 0
    ? rules
    : [
        { tier_label: '1K', max_pixels: 1048576, price: go.image_price_1k },
        { tier_label: '2K', max_pixels: 2359296, price: go.image_price_2k },
        { tier_label: '4K', max_pixels: null, price: go.image_price_4k },
      ]
  return source.map((rule) => ({
    tier_label: rule.tier_label,
    max_pixels: rule.max_pixels ?? '',
    price: rule.price ?? '',
  }))
}

function collectImageQualityPrices(): Record<string, number> | null {
  const out: Record<string, number> = {}
  for (const quality of imageQualities) {
    const raw = form.image_quality_prices[quality]
    if (raw !== '') out[quality] = Number(raw)
  }
  return Object.keys(out).length > 0 ? out : null
}

function collectImageQualityMultipliers(): Record<string, number> | null {
  const out: Record<string, number> = {}
  for (const quality of imageQualities) {
    const raw = form.image_quality_multipliers[quality]
    if (raw !== '') out[quality] = Number(raw)
  }
  return Object.keys(out).length > 0 ? out : null
}

function collectImageTierRules(): ImageTierRule[] {
  return form.image_tier_rules.map((rule) => ({
    tier_label: rule.tier_label,
    max_pixels: rule.max_pixels === '' ? null : Number(rule.max_pixels),
    price: rule.price === '' ? null : Number(rule.price),
  }))
}

async function loadDetail() {
  if (!props.model) return
  loading.value = true
  try {
    detail.value = await adminAPI.modelPricing.getDetail(props.model)
    // Populate form from existing override
    const go = detail.value.global_override
    if (go) {
      form.enabled = go.enabled
      form.billing_mode = go.billing_mode || 'token'
      form.input_price = go.input_price != null ? perTokenToMTok(go.input_price) ?? '' : ''
      form.output_price = go.output_price != null ? perTokenToMTok(go.output_price) ?? '' : ''
      form.cache_write_price = go.cache_write_price != null ? perTokenToMTok(go.cache_write_price) ?? '' : ''
      form.cache_read_price = go.cache_read_price != null ? perTokenToMTok(go.cache_read_price) ?? '' : ''
      form.image_output_price = go.image_output_price != null ? perTokenToMTok(go.image_output_price) ?? '' : ''
      form.per_request_price = go.per_request_price != null ? go.per_request_price : ''
      form.image_price_1k = go.image_price_1k != null ? go.image_price_1k : ''
      form.image_price_2k = go.image_price_2k != null ? go.image_price_2k : ''
      form.image_price_4k = go.image_price_4k != null ? go.image_price_4k : ''
      form.image_billing_strategy = go.image_billing_strategy || 'tier'
      form.image_megapixel_price = go.image_megapixel_price != null ? go.image_megapixel_price : ''
      resetImageQualityPrices(go.image_quality_prices || null)
      resetImageQualityMultipliers(go.image_quality_multipliers || null)
      form.image_tier_rules = buildTierRuleForm(go.image_tier_rules, go)
      form.provider = go.provider
      form.notes = go.notes
      form.display_input_price = go.display_input_price != null ? perTokenToMTok(go.display_input_price) ?? '' : ''
      form.display_output_price = go.display_output_price != null ? perTokenToMTok(go.display_output_price) ?? '' : ''
      form.display_cache_read_price = go.display_cache_read_price != null ? perTokenToMTok(go.display_cache_read_price) ?? '' : ''
      form.display_cache_creation_price = go.display_cache_creation_price != null ? perTokenToMTok(go.display_cache_creation_price) ?? '' : ''
      form.show_on_pricing_page = Boolean(go.show_on_pricing_page)
    } else {
      form.enabled = true
      form.billing_mode = 'token'
      form.input_price = ''
      form.output_price = ''
      form.cache_write_price = ''
      form.cache_read_price = ''
      form.image_output_price = ''
      form.per_request_price = ''
      form.image_price_1k = ''
      form.image_price_2k = ''
      form.image_price_4k = ''
      form.image_billing_strategy = 'tier'
      form.image_megapixel_price = ''
      resetImageQualityPrices(null)
      resetImageQualityMultipliers(null)
      form.image_tier_rules = defaultTierRules()
      form.provider = detail.value.provider || ''
      form.notes = ''
      form.display_input_price = ''
      form.display_output_price = ''
      form.display_cache_read_price = ''
      form.display_cache_creation_price = ''
      form.show_on_pricing_page = false
    }
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!detail.value) return
  saving.value = true
  try {
    const perReqVal = form.per_request_price === '' ? null : Number(form.per_request_price)
    const payload = {
      model: props.model,
      provider: form.provider,
      billing_mode: form.billing_mode,
      input_price: mTokToPerToken(form.input_price),
      output_price: mTokToPerToken(form.output_price),
      cache_write_price: mTokToPerToken(form.cache_write_price),
      cache_read_price: mTokToPerToken(form.cache_read_price),
      image_output_price: mTokToPerToken(form.image_output_price),
      per_request_price: perReqVal,
      image_price_1k: form.image_tier_rules[0]?.price === '' ? null : Number(form.image_tier_rules[0]?.price ?? 0),
      image_price_2k: form.image_tier_rules[1]?.price === '' ? null : Number(form.image_tier_rules[1]?.price ?? 0),
      image_price_4k: form.image_tier_rules[2]?.price === '' ? null : Number(form.image_tier_rules[2]?.price ?? 0),
      image_billing_strategy: form.image_billing_strategy,
      image_megapixel_price: form.image_megapixel_price === '' ? null : Number(form.image_megapixel_price),
      image_quality_prices: collectImageQualityPrices(),
      image_quality_multipliers: collectImageQualityMultipliers(),
      image_tier_rules: collectImageTierRules(),
      enabled: form.enabled,
      notes: form.notes,
      display_input_price: mTokToPerToken(form.display_input_price),
      display_output_price: mTokToPerToken(form.display_output_price),
      display_cache_read_price: mTokToPerToken(form.display_cache_read_price),
      display_cache_creation_price: mTokToPerToken(form.display_cache_creation_price),
      show_on_pricing_page: form.show_on_pricing_page,
    }

    if (detail.value.global_override) {
      await adminAPI.modelPricing.updateOverride(detail.value.global_override.id, payload)
    } else {
      await adminAPI.modelPricing.createOverride(payload)
    }
    appStore.showSuccess(t('common.saved'))
    emit('saved')
    emit('close')
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

function applySuggested() {
  const sp = detail.value?.suggested_prices
  if (!sp) return
  // 把建议价按字段写入表单（per-token → MTok）。不立即提交，让管理员确认后手动保存。
  const put = (val: number): string | number => (val > 0 ? (perTokenToMTok(val) ?? '') : '')
  form.input_price = put(sp.input_price)
  form.output_price = put(sp.output_price)
  form.cache_write_price = put(sp.cache_write_price)
  form.cache_read_price = put(sp.cache_read_price)
  form.image_output_price = put(sp.image_output_price)
}

/**
 * 有可用的展示价建议源：LiteLLM 价格、建议价、或已填写的计费价格任一存在即可。
 */
const hasDisplaySuggestion = computed(() => {
  if (!detail.value) return false
  return !!(detail.value.litellm_prices || detail.value.suggested_prices || form.input_price || form.output_price)
})

/**
 * 将当前生效的计费价格填入展示价字段。
 * 优先级：表单已填的计费价 > LiteLLM > 建议价。
 * 倍率默认 1，缓存转移比例默认 0.1。
 */
function applyDisplaySuggested() {
  const d = detail.value
  if (!d) return

  const lp = d.litellm_prices
  const sp = d.suggested_prices

  const inputPerToken = lp?.input_price ?? sp?.input_price ?? null
  const outputPerToken = lp?.output_price ?? sp?.output_price ?? null
  const cacheReadPerToken = lp?.cache_read_price ?? sp?.cache_read_price ?? null
  const cacheWritePerToken = lp?.cache_write_price ?? sp?.cache_write_price ?? null

  form.display_input_price = inputPerToken != null ? (perTokenToMTok(inputPerToken) ?? '') : ''
  form.display_output_price = outputPerToken != null ? (perTokenToMTok(outputPerToken) ?? '') : ''
  form.display_cache_read_price = cacheReadPerToken != null ? (perTokenToMTok(cacheReadPerToken) ?? '') : ''
  form.display_cache_creation_price = cacheWritePerToken != null ? (perTokenToMTok(cacheWritePerToken) ?? '') : ''
}

async function handleDelete() {
  if (!detail.value?.global_override) return
  if (!confirm(t('admin.modelPricing.confirmDeleteOverride'))) return
  saving.value = true
  try {
    await adminAPI.modelPricing.deleteOverride(detail.value.global_override.id)
    appStore.showSuccess(t('common.deleted'))
    emit('saved')
    emit('close')
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

watch(() => props.show, (val) => {
  if (val && props.model) loadDetail()
})
</script>
