<template>
    <div class="space-y-4" @keydown.enter.prevent>
      <div>
        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.inflightReservation.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.inflightReservation.description') }}
        </p>
      </div>

      <p v-if="loadError" class="text-sm text-red-600 dark:text-red-400" data-test="inflight-load-error">
        {{ loadError }}
      </p>

      <div v-else class="space-y-4">
        <section class="card space-y-4 p-4">
          <p class="text-xs text-gray-500 dark:text-gray-400" data-test="inflight-source">
            {{ sourceLabel }}
          </p>
          <p
            v-if="form.simple_mode"
            class="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
            data-test="inflight-simple-mode"
          >
            {{ t('admin.inflightReservation.simpleMode') }}
          </p>

          <label class="flex items-center justify-between gap-4">
            <span>
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.enabled') }}</span>
              <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.inflightReservation.enabledHint') }}</span>
            </span>
            <Toggle v-model="form.enabled" data-test="inflight-enabled" />
          </label>

          <fieldset class="space-y-2">
            <legend class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.inputPricing') }}</legend>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.input_pricing" type="radio" value="full" data-test="inflight-pricing-full" />
              <span>{{ t('admin.inflightReservation.pricingFull') }}</span>
            </label>
            <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input v-model="form.input_pricing" type="radio" value="cache_tail" data-test="inflight-pricing-cache-tail" />
              <span>{{ t('admin.inflightReservation.pricingCacheTail') }}</span>
            </label>
          </fieldset>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.fullPriceTokens') }}</span>
              <input v-model.number="form.full_price_input_tokens" type="number" min="0" step="1" class="field" :disabled="form.input_pricing !== 'cache_tail'" data-test="inflight-full-price-tokens" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.bytesPerToken') }}</span>
              <input v-model.number="form.bytes_per_token" type="number" min="1" max="64" step="1" class="field" data-test="inflight-bytes-per-token" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.defaultMaxTokens') }}</span>
              <input v-model.number="form.default_max_tokens" type="number" min="1" step="1" class="field" data-test="inflight-default-max-tokens" />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.inflightReservation.defaultMaxTokensHint') }}</span>
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.maxOutputTokens') }}</span>
              <input v-model.number="form.max_output_tokens" type="number" min="1" step="1" class="field" data-test="inflight-max-output" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.maxInputTokens') }}</span>
              <input v-model.number="form.max_input_tokens" type="number" min="1" step="1" class="field" data-test="inflight-max-input" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.maxReservation') }}</span>
              <input v-model.number="form.max_reservation_amount" type="number" min="0" step="0.01" class="field" data-test="inflight-max-reservation" />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.inflightReservation.maxReservationHint') }}</span>
            </label>
            <label class="block text-sm">
              <span class="mb-1 block font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.ttl') }}</span>
              <input v-model.number="form.ttl_seconds" type="number" min="30" max="86400" step="1" class="field" data-test="inflight-ttl" />
            </label>
          </div>

          <label class="flex items-center justify-between gap-4">
            <span>
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.failClosed') }}</span>
              <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.inflightReservation.failClosedHint') }}</span>
            </span>
            <Toggle v-model="form.fail_closed_on_unpriced" data-test="inflight-fail-closed" />
          </label>
        </section>

        <section class="card space-y-3 p-4">
          <h2 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.inflightReservation.previewTitle') }}</h2>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.inflightReservation.previewHint') }}</p>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewBodyBytes') }}</span>
              <input v-model.number="preview.bodyBytes" type="number" min="0" step="1" class="field" data-test="inflight-preview-bytes" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewMaxTokens') }}</span>
              <input v-model.number="preview.maxTokens" type="number" min="0" step="1" class="field" data-test="inflight-preview-max-tokens" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewInputPrice') }}</span>
              <input v-model.number="preview.inputPerMillion" type="number" min="0" step="0.01" class="field" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewCachePrice') }}</span>
              <input v-model.number="preview.cachePerMillion" type="number" min="0" step="0.01" class="field" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewOutputPrice') }}</span>
              <input v-model.number="preview.outputPerMillion" type="number" min="0" step="0.01" class="field" />
            </label>
            <label class="block text-sm">
              <span class="mb-1 block text-gray-700 dark:text-gray-200">{{ t('admin.inflightReservation.previewRate') }}</span>
              <input v-model.number="preview.rate" type="number" min="0" step="0.01" class="field" />
            </label>
          </div>
          <p class="text-sm text-gray-800 dark:text-gray-100" data-test="inflight-preview-amount">
            {{ previewText }}
          </p>
        </section>

        <div class="flex justify-end">
          <button type="button" class="btn btn-primary" :disabled="saving || loading" data-test="inflight-save" @click="save">
            {{ saving ? t('admin.inflightReservation.saving') : t('admin.inflightReservation.save') }}
          </button>
        </div>
      </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { InflightReservationSettings } from '@/api/admin/inflightReservation'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const loadError = ref('')

const form = reactive<InflightReservationSettings>({
  enabled: true,
  ttl_seconds: 900,
  bytes_per_token: 4,
  default_max_tokens: 8192,
  max_output_tokens: 128000,
  max_input_tokens: 200000,
  input_pricing: 'full',
  full_price_input_tokens: 4096,
  max_reservation_amount: 0,
  fail_closed_on_unpriced: false,
  source: 'config',
  simple_mode: false,
})

const preview = reactive({
  bodyBytes: 63853 * 4,
  maxTokens: 0,
  inputPerMillion: 20,
  cachePerMillion: 2,
  outputPerMillion: 100,
  rate: 0.45,
})

const sourceLabel = computed(() =>
  form.source === 'settings'
    ? t('admin.inflightReservation.sourceSettings')
    : t('admin.inflightReservation.sourceConfig'),
)

const previewText = computed(() => t('admin.inflightReservation.previewResult', previewResult.value))

const previewResult = computed(() => {
  const bytesPerToken = form.bytes_per_token > 0 ? form.bytes_per_token : 4
  let input = preview.bodyBytes > 0 ? Math.floor(preview.bodyBytes / bytesPerToken) : 0
  if (form.max_input_tokens > 0 && input > form.max_input_tokens) input = form.max_input_tokens
  let output = preview.maxTokens > 0 ? preview.maxTokens : form.default_max_tokens
  if (form.max_output_tokens > 0 && output > form.max_output_tokens) output = form.max_output_tokens
  let full = input
  let cache = 0
  if (form.input_pricing === 'cache_tail') {
    const keep = Math.max(0, form.full_price_input_tokens)
    full = Math.min(input, keep)
    cache = input - full
  }
  const inputPrice = preview.inputPerMillion / 1_000_000
  let cachePrice = preview.cachePerMillion / 1_000_000
  if (cache > 0 && cachePrice <= 0) cachePrice = inputPrice
  const outputPrice = preview.outputPerMillion / 1_000_000
  let amount = (full * inputPrice + cache * cachePrice + output * outputPrice) * preview.rate
  const capped = form.max_reservation_amount > 0 && amount > form.max_reservation_amount
  if (capped) amount = form.max_reservation_amount
  return {
    input,
    output,
    amount: amount.toFixed(4),
    capped: capped ? t('admin.inflightReservation.previewCapped') : '',
  }
})

function assignForm(settings: InflightReservationSettings) {
  form.enabled = settings.enabled
  form.ttl_seconds = settings.ttl_seconds
  form.bytes_per_token = settings.bytes_per_token
  form.default_max_tokens = settings.default_max_tokens
  form.max_output_tokens = settings.max_output_tokens
  form.max_input_tokens = settings.max_input_tokens
  form.input_pricing = settings.input_pricing === 'cache_tail' ? 'cache_tail' : 'full'
  form.full_price_input_tokens = settings.full_price_input_tokens
  form.max_reservation_amount = settings.max_reservation_amount
  form.fail_closed_on_unpriced = settings.fail_closed_on_unpriced
  form.source = settings.source
  form.simple_mode = settings.simple_mode
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    assignForm(await adminAPI.inflightReservation.get())
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.inflightReservation.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    assignForm(await adminAPI.inflightReservation.update({ ...form }))
    appStore.showSuccess(t('admin.inflightReservation.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.inflightReservation.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.field {
  width: 100%;
  border-radius: 0.5rem;
  border: 1px solid rgb(209 213 219);
  background: white;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
}
:global(.dark) .field {
  border-color: rgb(75 85 99);
  background: rgb(31 41 55);
  color: white;
}
</style>
