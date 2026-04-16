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
}>()

const emit = defineEmits<{
  close: []
  saved: [payload: { mode: 'add' | 'edit' | 'delete'; from: string; to?: string }]
}>()

const { t } = useI18n()
const fromRef = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const error = ref('')

const form = reactive<{ from: string; to: string }>({ from: '', to: '' })

const canSave = computed(() => form.from.trim() !== '' && form.to.trim() !== '')

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
  if (props.mode === 'edit') {
    form.from = props.originalFrom ?? ''
    form.to = props.originalTo ?? ''
  } else {
    form.from = ''
    form.to = ''
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
      // add 模式聚焦 from，edit 模式聚焦 to（更常见的修改对象）
      const targetRef = fromRef.value
      if (targetRef) {
        if (props.mode === 'edit') {
          // 选中已有值便于覆盖
          targetRef.focus({ preventScroll: true })
          targetRef.select()
        } else {
          targetRef.focus({ preventScroll: true })
        }
      }
    }
  }
)

async function handleSave() {
  if (!canSave.value) return
  const from = form.from.trim()
  const to = form.to.trim()
  if (!from || !to) return

  saving.value = true
  error.value = ''
  try {
    // 整表读 → 修改 → 整表写
    const current = await adminAPI.accounts.getAntigravityDefaultModelMapping()
    const next = { ...current }

    if (props.mode === 'edit') {
      const origFrom = props.originalFrom ?? ''
      if (origFrom && origFrom !== from) {
        // 改名：先删旧键
        delete next[origFrom]
      }
    }
    // edit 模式下如果 add 场景下的 from 已存在，会直接覆盖旧 value——这是合理行为
    next[from] = to

    await adminAPI.accounts.updateAntigravityDefaultModelMapping(next)
    emit('saved', { mode: props.mode, from, to })
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
    const current = await adminAPI.accounts.getAntigravityDefaultModelMapping()
    const next = { ...current }
    delete next[origFrom]
    await adminAPI.accounts.updateAntigravityDefaultModelMapping(next)
    emit('saved', { mode: 'delete', from: origFrom })
    emit('close')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    error.value = msg || t('common.error')
  } finally {
    saving.value = false
  }
}
</script>
