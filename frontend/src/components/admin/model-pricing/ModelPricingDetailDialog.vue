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
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" v-model="form.enabled" class="rounded" />
            {{ t('admin.modelPricing.enabled') }}
          </label>
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

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
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
          <div>
            <label class="mb-1 block text-xs text-gray-500">Provider</label>
            <input v-model="form.provider" type="text" class="input text-sm w-full" :placeholder="detail.provider || ''" />
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

const form = reactive({
  enabled: true,
  input_price: '' as string | number,
  output_price: '' as string | number,
  cache_write_price: '' as string | number,
  cache_read_price: '' as string | number,
  image_output_price: '' as string | number,
  provider: '',
  notes: '',
})

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

async function loadDetail() {
  if (!props.model) return
  loading.value = true
  try {
    detail.value = await adminAPI.modelPricing.getDetail(props.model)
    // Populate form from existing override
    const go = detail.value.global_override
    if (go) {
      form.enabled = go.enabled
      form.input_price = go.input_price != null ? perTokenToMTok(go.input_price) ?? '' : ''
      form.output_price = go.output_price != null ? perTokenToMTok(go.output_price) ?? '' : ''
      form.cache_write_price = go.cache_write_price != null ? perTokenToMTok(go.cache_write_price) ?? '' : ''
      form.cache_read_price = go.cache_read_price != null ? perTokenToMTok(go.cache_read_price) ?? '' : ''
      form.image_output_price = go.image_output_price != null ? perTokenToMTok(go.image_output_price) ?? '' : ''
      form.provider = go.provider
      form.notes = go.notes
    } else {
      form.enabled = true
      form.input_price = ''
      form.output_price = ''
      form.cache_write_price = ''
      form.cache_read_price = ''
      form.image_output_price = ''
      form.provider = detail.value.provider || ''
      form.notes = ''
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
    const payload = {
      model: props.model,
      provider: form.provider,
      billing_mode: 'token' as const,
      input_price: mTokToPerToken(form.input_price),
      output_price: mTokToPerToken(form.output_price),
      cache_write_price: mTokToPerToken(form.cache_write_price),
      cache_read_price: mTokToPerToken(form.cache_read_price),
      image_output_price: mTokToPerToken(form.image_output_price),
      enabled: form.enabled,
      notes: form.notes,
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
