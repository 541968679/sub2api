<template>
  <!-- 使用 Teleport 挂到 body 避免表格 overflow 裁切 -->
  <Teleport to="body">
    <!-- 背景点击层：关闭 popover -->
    <div
      v-if="show"
      class="fixed inset-0 z-40"
      @click="$emit('close')"
    ></div>

    <!-- Popover 面板 -->
    <div
      v-if="show"
      ref="panelRef"
      class="fixed z-50 w-80 rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
      :style="panelStyle"
      @click.stop
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-gray-200 px-4 py-2.5 dark:border-gray-700">
        <div class="flex items-center gap-2 min-w-0">
          <span class="text-xs text-gray-500 shrink-0">{{ t('admin.modelPricing.inlineEditTitle') }}</span>
          <span class="truncate font-mono text-sm text-gray-900 dark:text-white" :title="model">{{ model }}</span>
        </div>
        <button
          @click="$emit('close')"
          class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
          :title="t('common.close')"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Body -->
      <div class="space-y-3 px-4 py-3">
        <label class="flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300">
          <input type="checkbox" v-model="form.enabled" class="rounded" />
          {{ t('admin.modelPricing.enabled') }}
        </label>

        <div v-for="field in fields" :key="field.key" class="space-y-0.5">
          <label class="flex items-baseline justify-between text-xs text-gray-500 dark:text-gray-400">
            <span>{{ field.label }}</span>
            <span
              v-if="field.baselineText"
              class="text-gray-400 dark:text-gray-500"
            >
              {{ t('admin.modelPricing.baselinePrefix') }} {{ field.baselineText }}
            </span>
          </label>
          <input
            :ref="(el) => setFieldRef(el, field.key)"
            v-model="form[field.key]"
            type="number"
            step="any"
            class="input w-full text-sm"
            :placeholder="field.placeholder"
            @keydown.enter.prevent="handleSave"
          />
        </div>

        <p v-if="error" class="text-xs text-rose-600 dark:text-rose-400">{{ error }}</p>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-between gap-2 border-t border-gray-200 bg-gray-50/50 px-4 py-2.5 dark:border-gray-700 dark:bg-gray-800/50">
        <button
          @click="$emit('openFullDialog')"
          class="text-xs text-primary-600 hover:underline dark:text-primary-400"
          :disabled="saving"
        >
          {{ t('admin.modelPricing.openFullDialog') }}
        </button>
        <div class="flex gap-2">
          <button
            v-if="existingOverrideId != null"
            @click="handleDelete"
            class="btn btn-secondary text-xs text-red-600 hover:text-red-700"
            :disabled="saving"
          >
            {{ t('common.delete') }}
          </button>
          <button
            @click="handleSave"
            class="btn btn-primary text-xs"
            :disabled="saving"
          >
            <span v-if="saving" class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            {{ t('admin.modelPricing.saveInline') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GlobalOverride, LiteLLMPrices } from '@/api/admin/modelPricing'
import { mTokToPerToken, perTokenToMTok } from '@/components/admin/channel/types'

const props = defineProps<{
  show: boolean
  /** 锚点元素，用于计算 popover 位置 */
  anchor: HTMLElement | null
  /** 模型名 */
  model: string
  /** 已有的全局覆盖（null 表示新建） */
  existingOverride: GlobalOverride | null
  /** LiteLLM 基准价（可选，用于 placeholder） */
  litellmBaseline: LiteLLMPrices | null
  /** 触发时聚焦的字段（即用户点击的那个价格字段） */
  focusField?: 'input' | 'output' | 'cache_write' | 'cache_read'
}>()

const emit = defineEmits<{
  close: []
  /** 保存成功后回传的数据，父组件在本地 items 里就地替换 */
  saved: [payload: { model: string; override: GlobalOverride | null; wasCreate: boolean; wasDelete: boolean }]
  /** 切换到完整对话框 */
  openFullDialog: []
}>()

const { t } = useI18n()
const panelRef = ref<HTMLElement | null>(null)
const saving = ref(false)
const error = ref('')

// 每个字段的 input ref 用于 focus
const fieldRefs = ref<Record<string, HTMLInputElement | null>>({})
function setFieldRef(el: unknown, key: string) {
  if (el && (el as HTMLInputElement).tagName === 'INPUT') {
    fieldRefs.value[key] = el as HTMLInputElement
  }
}

type FieldKey = 'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price'

const form = reactive<{
  enabled: boolean
  input_price: number | string
  output_price: number | string
  cache_write_price: number | string
  cache_read_price: number | string
}>({
  enabled: true,
  input_price: '',
  output_price: '',
  cache_write_price: '',
  cache_read_price: '',
})

const fields = computed(() => [
  {
    key: 'input_price' as FieldKey,
    label: `${t('admin.modelPricing.inputPrice')} ($/MTok)`,
    baselineText: baselineMtok('input_price'),
    placeholder: baselineMtok('input_price') || '',
  },
  {
    key: 'output_price' as FieldKey,
    label: `${t('admin.modelPricing.outputPrice')} ($/MTok)`,
    baselineText: baselineMtok('output_price'),
    placeholder: baselineMtok('output_price') || '',
  },
  {
    key: 'cache_write_price' as FieldKey,
    label: `${t('admin.modelPricing.cacheWritePrice')} ($/MTok)`,
    baselineText: baselineMtok('cache_write_price'),
    placeholder: baselineMtok('cache_write_price') || '',
  },
  {
    key: 'cache_read_price' as FieldKey,
    label: `${t('admin.modelPricing.cacheReadPrice')} ($/MTok)`,
    baselineText: baselineMtok('cache_read_price'),
    placeholder: baselineMtok('cache_read_price') || '',
  },
])

function baselineMtok(key: keyof LiteLLMPrices): string {
  const base = props.litellmBaseline
  if (!base) return ''
  const v = base[key]
  if (typeof v !== 'number' || v <= 0) return ''
  const m = perTokenToMTok(v)
  return m !== null ? `$${m}` : ''
}

const existingOverrideId = computed(() => props.existingOverride?.id ?? null)

// Popover 位置计算：锚定在 anchor 正下方，横向尽量靠左边对齐但不溢出
const panelStyle = ref<{ top?: string; left?: string }>({})
function updatePosition() {
  if (!props.anchor) return
  const rect = props.anchor.getBoundingClientRect()
  const panelWidth = 320 // w-80
  const panelHeightEst = 380 // 估算
  const gap = 4

  let top = rect.bottom + gap
  let left = rect.left

  // 右边溢出：右对齐到 anchor
  if (left + panelWidth > window.innerWidth - 8) {
    left = Math.max(8, window.innerWidth - panelWidth - 8)
  }
  // 下方空间不够：显示在 anchor 上方
  if (top + panelHeightEst > window.innerHeight - 8) {
    top = Math.max(8, rect.top - panelHeightEst - gap)
  }

  panelStyle.value = { top: `${top}px`, left: `${left}px` }
}

function resetForm() {
  const go = props.existingOverride
  if (go) {
    form.enabled = go.enabled
    form.input_price = go.input_price != null ? perTokenToMTok(go.input_price) ?? '' : ''
    form.output_price = go.output_price != null ? perTokenToMTok(go.output_price) ?? '' : ''
    form.cache_write_price = go.cache_write_price != null ? perTokenToMTok(go.cache_write_price) ?? '' : ''
    form.cache_read_price = go.cache_read_price != null ? perTokenToMTok(go.cache_read_price) ?? '' : ''
  } else {
    form.enabled = true
    form.input_price = ''
    form.output_price = ''
    form.cache_write_price = ''
    form.cache_read_price = ''
  }
  error.value = ''
}

watch(
  () => props.show,
  async (val) => {
    if (val) {
      resetForm()
      await nextTick()
      updatePosition()
      // 聚焦到用户点击的那个字段
      const key = props.focusField ? `${props.focusField}_price` : 'input_price'
      const el = fieldRefs.value[key]
      if (el) el.focus({ preventScroll: true })
    }
  }
)

async function handleSave() {
  saving.value = true
  error.value = ''
  try {
    // 保留原 override 的 provider / notes / image_output_price / per_request_price 等字段
    const go = props.existingOverride
    const payload = {
      model: props.model,
      provider: go?.provider ?? '',
      billing_mode: (go?.billing_mode ?? 'token') as 'token' | 'per_request' | 'image',
      input_price: mTokToPerToken(form.input_price),
      output_price: mTokToPerToken(form.output_price),
      cache_write_price: mTokToPerToken(form.cache_write_price),
      cache_read_price: mTokToPerToken(form.cache_read_price),
      image_output_price: go?.image_output_price ?? null,
      per_request_price: go?.per_request_price ?? null,
      enabled: form.enabled,
      notes: go?.notes ?? '',
    }

    let result: GlobalOverride
    let wasCreate = false
    if (go) {
      result = await adminAPI.modelPricing.updateOverride(go.id, payload)
    } else {
      result = await adminAPI.modelPricing.createOverride(payload)
      wasCreate = true
    }

    emit('saved', { model: props.model, override: result, wasCreate, wasDelete: false })
    emit('close')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    error.value = msg || t('common.error')
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  const go = props.existingOverride
  if (!go) return
  if (!confirm(t('admin.modelPricing.confirmDeleteOverride'))) return
  saving.value = true
  error.value = ''
  try {
    await adminAPI.modelPricing.deleteOverride(go.id)
    emit('saved', { model: props.model, override: null, wasCreate: false, wasDelete: true })
    emit('close')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    error.value = msg || t('common.error')
  } finally {
    saving.value = false
  }
}
</script>
