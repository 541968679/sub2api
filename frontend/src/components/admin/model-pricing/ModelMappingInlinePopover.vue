<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-40" @click="$emit('close')"></div>

    <div
      v-if="show"
      class="fixed z-50 w-96 rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
      :style="panelStyle"
      @click.stop
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-gray-200 px-4 py-2.5 dark:border-gray-700">
        <span class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ mode === 'add' ? t('admin.modelPricing.addMapping') : t('admin.modelPricing.editMapping') }}
        </span>
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
        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.modelPricing.providerLabel') }}
          </label>
          <select v-model="form.platform" class="input w-full text-sm">
            <option v-for="option in platformOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>

        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.modelPricing.requestedModelName') }}
          </label>
          <input
            ref="fromRef"
            v-model="form.from"
            type="text"
            class="input w-full font-mono text-sm"
            :placeholder="t('admin.modelPricing.mappingFromPlaceholder')"
            @keydown.enter.prevent="handleSave"
          />
        </div>

        <div>
          <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.modelPricing.upstreamModelName') }}
          </label>
          <input
            ref="toRef"
            v-model="form.to"
            type="text"
            class="input w-full font-mono text-sm"
            :placeholder="t('admin.modelPricing.mappingToPlaceholder')"
            @keydown.enter.prevent="handleSave"
          />
          <p class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">
            {{ t('admin.modelPricing.mappingToHint') }}
          </p>
        </div>

        <p v-if="error" class="text-xs text-rose-600 dark:text-rose-400">{{ error }}</p>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-between gap-2 border-t border-gray-200 bg-gray-50/50 px-4 py-2.5 dark:border-gray-700 dark:bg-gray-800/50">
        <div>
          <button
            v-if="mode === 'edit'"
            @click="handleDelete"
            class="btn btn-secondary text-xs text-red-600 hover:text-red-700"
            :disabled="saving"
          >
            {{ t('admin.modelPricing.deleteMapping') }}
          </button>
        </div>
        <div class="flex gap-2">
          <button @click="$emit('close')" class="btn btn-secondary text-xs" :disabled="saving">
            {{ t('common.cancel') }}
          </button>
          <button @click="handleSave" class="btn btn-primary text-xs" :disabled="saving || !canSave">
            <span v-if="saving" class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
            {{ t('common.save') }}
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
import {
  MODEL_PRICING_PROVIDER_OPTIONS,
  normalizeModelPricingProvider,
  type ModelPricingProvider,
} from './modelPricingOptions'

type Mode = 'add' | 'edit'

const props = defineProps<{
  show: boolean
  /** 锚点元素，用于定位 */
  anchor: HTMLElement | null
  /** add：新增映射；edit：编辑已有映射 */
  mode: Mode
  /** edit 模式必填：原始请求名（作为定位 key） */
  originalFrom?: string
  /** edit 模式必填：原始上游名（初始化 form.to） */
  originalTo?: string
  /** 当前映射所属平台 */
  platform?: string
  /** add 模式可选：预填请求名（从直通行"添加映射"进入时为模型名） */
  initialFrom?: string
  /** add 模式可选：预填上游名 */
  initialTo?: string
}>()

const emit = defineEmits<{
  close: []
  saved: [payload: { mode: 'add' | 'edit' | 'delete'; platform: string; from: string; to?: string }]
}>()

const { t } = useI18n()
const fromRef = ref<HTMLInputElement | null>(null)
const toRef = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const error = ref('')

const platformOptions = MODEL_PRICING_PROVIDER_OPTIONS

const form = reactive<{ platform: ModelPricingProvider; from: string; to: string }>({ platform: 'antigravity', from: '', to: '' })

const canSave = computed(() => form.from.trim() !== '' && form.to.trim() !== '')

function normalizePlatform(platform?: string): ModelPricingProvider {
  return normalizeModelPricingProvider(platform) || 'antigravity'
}

async function moveBillingObjectOverride(
  originalPlatform: ModelPricingProvider,
  originalFrom: string,
  nextPlatform: ModelPricingProvider,
  nextFrom: string
) {
  if (!originalFrom || (originalPlatform === nextPlatform && originalFrom === nextFrom)) {
    return
  }
  const oldObjects = await adminAPI.accounts.getPlatformDefaultModelMappingBillingObjects(originalPlatform)
  const oldObject = oldObjects[originalFrom]
  if (oldObject) {
    const nextOldObjects = { ...oldObjects }
    delete nextOldObjects[originalFrom]
    await adminAPI.accounts.updatePlatformDefaultModelMappingBillingObjects(originalPlatform, nextOldObjects)
  }
  if (oldObject) {
    const newObjects = originalPlatform === nextPlatform
      ? { ...oldObjects }
      : await adminAPI.accounts.getPlatformDefaultModelMappingBillingObjects(nextPlatform)
    delete newObjects[originalFrom]
    newObjects[nextFrom] = oldObject
    await adminAPI.accounts.updatePlatformDefaultModelMappingBillingObjects(nextPlatform, newObjects)
  }
}

async function deleteBillingObjectOverride(platform: ModelPricingProvider, from: string) {
  if (!from) return
  const objects = await adminAPI.accounts.getPlatformDefaultModelMappingBillingObjects(platform)
  if (!(from in objects)) return
  const nextObjects = { ...objects }
  delete nextObjects[from]
  await adminAPI.accounts.updatePlatformDefaultModelMappingBillingObjects(platform, nextObjects)
}

// Popover 位置
const panelStyle = ref<{ top?: string; left?: string }>({})
function updatePosition() {
  if (!props.anchor) return
  const rect = props.anchor.getBoundingClientRect()
  const panelWidth = 384 // w-96
  const panelHeightEst = 280
  const gap = 4

  let top = rect.bottom + gap
  let left = rect.left

  if (left + panelWidth > window.innerWidth - 8) {
    left = Math.max(8, window.innerWidth - panelWidth - 8)
  }
  if (top + panelHeightEst > window.innerHeight - 8) {
    top = Math.max(8, rect.top - panelHeightEst - gap)
  }

  panelStyle.value = { top: `${top}px`, left: `${left}px` }
}

function resetForm() {
  form.platform = normalizePlatform(props.platform)
  if (props.mode === 'edit') {
    form.from = props.originalFrom ?? ''
    form.to = props.originalTo ?? ''
  } else {
    form.from = props.initialFrom ?? ''
    form.to = props.initialTo ?? ''
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
      // 预填了请求名（直通行"添加映射"）或 edit 模式时聚焦上游名；
      // 空白新增聚焦请求名。
      if (props.mode === 'edit' || form.from) {
        const target = toRef.value
        if (target) {
          target.focus({ preventScroll: true })
          target.select()
        }
      } else {
        fromRef.value?.focus({ preventScroll: true })
      }
    }
  }
)

async function handleSave() {
  if (!canSave.value) return
  const from = form.from.trim()
  const to = form.to.trim()
  const platform = normalizePlatform(form.platform)
  if (!platform || !from || !to) return

  saving.value = true
  error.value = ''
  try {
    // 整表读 → 修改 → 整表写
    const originalPlatform = normalizePlatform(props.platform)
    if (props.mode === 'edit' && originalPlatform !== platform) {
      const oldMapping = await adminAPI.accounts.getPlatformDefaultModelMapping(originalPlatform)
      const oldNext = { ...oldMapping }
      const origFrom = props.originalFrom ?? ''
      if (origFrom) {
        delete oldNext[origFrom]
        await adminAPI.accounts.updatePlatformDefaultModelMapping(originalPlatform, oldNext)
      }
    }

    const current = await adminAPI.accounts.getPlatformDefaultModelMapping(platform)
    const next = { ...current }

    if (props.mode === 'edit') {
      const origFrom = props.originalFrom ?? ''
      if (origFrom && originalPlatform === platform && origFrom !== from) {
        // 改名：先删旧键
        delete next[origFrom]
      }
    }
    // edit 模式下如果 add 场景下的 from 已存在，会直接覆盖旧 value——这是合理行为
    next[from] = to

    await adminAPI.accounts.updatePlatformDefaultModelMapping(platform, next)
    if (props.mode === 'edit') {
      await moveBillingObjectOverride(originalPlatform, props.originalFrom ?? '', platform, from)
    }
    emit('saved', { mode: props.mode, platform, from, to })
    emit('close')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    error.value = msg || t('common.error')
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  const origFrom = props.originalFrom ?? ''
  if (!origFrom) return
  if (!confirm(t('admin.modelPricing.confirmDeleteMapping', { from: origFrom }))) return
  saving.value = true
  error.value = ''
  try {
    const platform = normalizePlatform(props.platform)
    const current = await adminAPI.accounts.getPlatformDefaultModelMapping(platform)
    const next = { ...current }
    delete next[origFrom]
    await adminAPI.accounts.updatePlatformDefaultModelMapping(platform, next)
    await deleteBillingObjectOverride(platform, origFrom)
    emit('saved', { mode: 'delete', platform, from: origFrom })
    emit('close')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    error.value = msg || t('common.error')
  } finally {
    saving.value = false
  }
}
</script>
