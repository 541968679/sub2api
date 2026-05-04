<template>
  <BaseDialog :show="show" :title="t('admin.users.modelPricingConfig')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-6">
      <!-- 用户信息头部 -->
      <div class="flex items-center gap-4 rounded-2xl bg-gradient-to-r from-primary-50 to-primary-100 p-5 dark:from-primary-900/30 dark:to-primary-800/20">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.users.modelPricingHint') }}</p>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-12">
        <svg class="h-10 w-10 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div v-else class="space-y-4">
        <!-- 添加按钮 -->
        <button
          class="flex items-center gap-2 rounded-lg border-2 border-dashed border-gray-300 px-4 py-2 text-sm text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-500 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
          @click="addOverride"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
          {{ t('admin.users.addModelOverride') }}
        </button>

        <!-- 覆盖列表 -->
        <div v-for="(item, idx) in overrides" :key="idx" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <div class="flex items-center justify-between mb-3">
            <div class="w-64">
              <Select
                v-model="item.model"
                :options="modelOptions"
                :placeholder="t('admin.users.modelNamePlaceholder')"
                searchable
              />
            </div>
            <div class="flex items-center gap-3">
              <label class="flex items-center gap-1.5 text-sm">
                <input v-model="item.enabled" type="checkbox" class="rounded text-primary-500" />
                {{ t('common.enabled') }}
              </label>
              <button class="text-red-500 hover:text-red-700 text-sm" @click="removeOverride(idx)">
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <!-- LiteLLM 标准价格参考 -->
          <div v-if="item.model && modelInfoMap.get(item.model)" class="mb-3 rounded-md bg-blue-50 dark:bg-blue-900/20 p-2 text-xs">
            <div class="font-medium text-blue-700 dark:text-blue-300 mb-1">{{ t('admin.users.litellmReference') }}</div>
            <div class="flex flex-wrap gap-x-4 gap-y-1 text-blue-600 dark:text-blue-400">
              <span>{{ t('admin.modelPricing.inputPrice') }}: {{ getSuggestedMTok(item, 'input_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.outputPrice') }}: {{ getSuggestedMTok(item, 'output_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.cacheWritePrice') }}: {{ getSuggestedMTok(item, 'cache_write_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.cacheReadPrice') }}: {{ getSuggestedMTok(item, 'cache_read_price') ?? '-' }}</span>
              <span class="text-blue-400">($/MTok)</span>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- 真实计费 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.billingPriceOverride') }}</h5>
                <button
                  v-if="item.model && modelInfoMap.get(item.model)"
                  @click="applySuggestedBilling(item)"
                  type="button"
                  class="text-[10px] text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 underline"
                >
                  {{ t('admin.users.applySuggested') }}
                </button>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.inputPrice') }}</label>
                  <input v-model.number="item.input_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.outputPrice') }}</label>
                  <input v-model.number="item.output_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheWritePrice') }}</label>
                  <input v-model.number="item.cache_write_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheReadPrice') }}</label>
                  <input v-model.number="item.cache_read_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
              </div>
            </div>

            <!-- 展示覆盖 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.displayPriceOverride') }}</h5>
                <button
                  v-if="item.model && modelInfoMap.get(item.model)"
                  @click="applySuggestedDisplay(item)"
                  type="button"
                  class="text-[10px] text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300 underline"
                >
                  {{ t('admin.users.applySuggested') }}
                </button>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayInputPrice') }}</label>
                  <input v-model.number="item.display_input_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayOutputPrice') }}</label>
                  <input v-model.number="item.display_output_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayCacheReadPrice') }}</label>
                  <input v-model.number="item.display_cache_read_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheTransferRatio') }}</label>
                  <input v-model.number="item.cache_transfer_ratio" type="number" step="any" min="0" max="1" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
              </div>
            </div>
          </div>

          <!-- 备注 -->
          <div class="mt-2">
            <input v-model="item.notes" type="text" :placeholder="t('admin.users.notesPlaceholder')"
              class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
          </div>
        </div>

        <div v-if="overrides.length === 0" class="py-8 text-center text-sm text-gray-400">
          {{ t('admin.users.noModelOverrides') }}
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          class="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-700 transition hover:bg-gray-50 dark:border-dark-500 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="$emit('close')"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          class="rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition hover:bg-primary-600 disabled:opacity-50"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { perTokenToMTok, mTokToPerToken } from '@/components/admin/channel/types'
import { adminAPI } from '@/api/admin'
import {
  getUserModelPricing,
  batchUpsertUserModelPricing,
  deleteUserModelPricing,
  type UserModelPricingOverride,
} from '@/api/admin/userModelPricing'

const { t } = useI18n()

interface OverrideRow {
  id?: number
  model: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null

  cache_transfer_ratio: number | null
  enabled: boolean
  notes: string
  _deleted?: boolean
}

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])

interface ModelInfo {
  model: string
  provider: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
}

const loading = ref(false)
const saving = ref(false)
const overrides = ref<OverrideRow[]>([])
const originalIds = ref<Set<number>>(new Set())
const availableModels = ref<ModelInfo[]>([])
const modelInfoMap = computed(() => {
  const m = new Map<string, ModelInfo>()
  for (const info of availableModels.value) {
    m.set(info.model, info)
  }
  return m
})

const modelOptions = computed(() => {
  const seen = new Set<string>()
  const opts: Array<{ value: string; label: string }> = []
  for (const m of availableModels.value) {
    if (seen.has(m.model)) continue
    seen.add(m.model)
    opts.push({
      value: m.model,
      label: m.provider ? `${m.model}  ·  ${m.provider}` : m.model,
    })
  }
  // Include any already-saved models that aren't in the list (e.g. retired models)
  for (const o of overrides.value) {
    if (o.model && !seen.has(o.model)) {
      seen.add(o.model)
      opts.push({ value: o.model, label: o.model })
    }
  }
  return opts
})

async function loadAvailableModels() {
  if (availableModels.value.length > 0) return
  try {
    const result = await adminAPI.modelPricing.list(1, 1000)
    availableModels.value = (result.items || []).map((i: any) => ({
      model: i.model,
      provider: i.provider || '',
      input_price: i.litellm_prices?.input_price ?? null,
      output_price: i.litellm_prices?.output_price ?? null,
      cache_write_price: i.litellm_prices?.cache_write_price ?? null,
      cache_read_price: i.litellm_prices?.cache_read_price ?? null,
    }))
  } catch (e) {
    console.error('[UserModelPricing] failed to load model list:', e)
  }
}

function toNullableNum(val: string | number | null | undefined): number | null {
  if (val === '' || val === null || val === undefined) return null
  const n = Number(val)
  return isNaN(n) ? null : n
}

/**
 * 获取选中模型的 LiteLLM 标准价格，格式化为 $/MTok 用于表单填充。
 * 若该字段没有 LiteLLM 价格则返回 null。
 */
function getSuggestedMTok(item: OverrideRow, field: 'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price'): number | null {
  const info = modelInfoMap.value.get(item.model)
  if (!info) return null
  const perToken = info[field]
  if (perToken == null) return null
  return perTokenToMTok(perToken) ?? null
}

function applySuggestedBilling(item: OverrideRow) {
  const fields: Array<'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price'> = [
    'input_price', 'output_price', 'cache_write_price', 'cache_read_price',
  ]
  for (const f of fields) {
    const v = getSuggestedMTok(item, f)
    if (v != null) item[f] = v
  }
}

function applySuggestedDisplay(item: OverrideRow) {
  const inputMTok = getSuggestedMTok(item, 'input_price')
  if (inputMTok != null) item.display_input_price = inputMTok
  const outputMTok = getSuggestedMTok(item, 'output_price')
  if (outputMTok != null) item.display_output_price = outputMTok
  const cacheReadMTok = getSuggestedMTok(item, 'cache_read_price')
  if (cacheReadMTok != null) item.display_cache_read_price = cacheReadMTok
}

watch(
  () => props.show,
  async (val) => {
    if (!val || !props.user) return
    loading.value = true
    try {
      await loadAvailableModels()
      const data = await getUserModelPricing(props.user.id)
      overrides.value = (data || []).map((o: UserModelPricingOverride) => ({
        id: o.id,
        model: o.model,
        input_price: perTokenToMTok(o.input_price) ?? null,
        output_price: perTokenToMTok(o.output_price) ?? null,
        cache_write_price: perTokenToMTok(o.cache_write_price) ?? null,
        cache_read_price: perTokenToMTok(o.cache_read_price) ?? null,
        display_input_price: perTokenToMTok(o.display_input_price) ?? null,
        display_output_price: perTokenToMTok(o.display_output_price) ?? null,
        display_cache_read_price: perTokenToMTok(o.display_cache_read_price) ?? null,
        cache_transfer_ratio: o.cache_transfer_ratio,
        enabled: o.enabled,
        notes: o.notes || '',
      }))
      originalIds.value = new Set((data || []).map((o: UserModelPricingOverride) => o.id))
    } catch {
      overrides.value = []
    } finally {
      loading.value = false
    }
  }
)

function addOverride() {
  overrides.value.push({
    model: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    display_input_price: null,
    display_output_price: null,
    display_cache_read_price: null,
    cache_transfer_ratio: null,
    enabled: true,
    notes: '',
  })
}

function removeOverride(idx: number) {
  overrides.value.splice(idx, 1)
}

async function save() {
  if (!props.user) return
  saving.value = true
  try {
    const userId = props.user.id
    const currentIds = new Set(overrides.value.filter((o) => o.id).map((o) => o.id!))
    for (const oldId of originalIds.value) {
      if (!currentIds.has(oldId)) {
        await deleteUserModelPricing(userId, oldId)
      }
    }

    const toUpsert = overrides.value
      .filter((o) => o.model.trim())
      .map((o) => ({
        model: o.model.trim(),
        input_price: mTokToPerToken(o.input_price),
        output_price: mTokToPerToken(o.output_price),
        cache_write_price: mTokToPerToken(o.cache_write_price),
        cache_read_price: mTokToPerToken(o.cache_read_price),
        display_input_price: mTokToPerToken(o.display_input_price),
        display_output_price: mTokToPerToken(o.display_output_price),
        display_cache_read_price: mTokToPerToken(o.display_cache_read_price),
        cache_transfer_ratio: toNullableNum(o.cache_transfer_ratio),
        enabled: o.enabled,
        notes: o.notes || '',
      }))

    // 前端去重：同一模型名多条记录时告警并阻止保存
    const modelCounts = new Map<string, number>()
    for (const o of toUpsert) {
      modelCounts.set(o.model, (modelCounts.get(o.model) || 0) + 1)
    }
    const dupes = Array.from(modelCounts.entries()).filter(([, c]) => c > 1).map(([m]) => m)
    if (dupes.length > 0) {
      alert(t('admin.users.duplicateModelError', { models: dupes.join(', ') }))
      saving.value = false
      return
    }
    if (toUpsert.length > 0) {
      await batchUpsertUserModelPricing(userId, toUpsert)
    }

    emit('success')
    emit('close')
  } catch (e) {
    console.error('[UserModelPricing] Save failed:', e)
  } finally {
    saving.value = false
  }
}
</script>
