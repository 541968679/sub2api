<template>
  <div class="inline-flex min-w-[3.5rem] items-center gap-1" @click.stop>
    <template v-if="editing">
      <input
        ref="inputRef"
        v-model="draft"
        type="number"
        :min="min"
        :step="step"
        class="w-16 rounded border border-primary-400 bg-white px-1.5 py-0.5 text-sm tabular-nums text-gray-900 shadow-sm focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-primary-500 dark:bg-dark-800 dark:text-white"
        :disabled="disabled"
        @keydown.enter.prevent="commit"
        @keydown.escape.prevent="cancel"
        @blur="commit"
      />
    </template>
    <button
      v-else
      type="button"
      class="group inline-flex items-center gap-1 rounded px-1 py-0.5 text-left text-sm tabular-nums text-gray-800 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-dark-700"
      :title="hint || t('admin.accounts.inlineEdit.hint')"
      :disabled="disabled"
      @click="startEdit"
    >
      <span class="font-medium">{{ displayValue }}</span>
      <svg
        class="h-3 w-3 shrink-0 text-gray-400 opacity-0 transition-opacity group-hover:opacity-100"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931z"
        />
      </svg>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(
  defineProps<{
    modelValue: number
    min?: number
    step?: number
    disabled?: boolean
    hint?: string
    blankWhenZero?: boolean
    allowDecimal?: boolean
  }>(),
  {
    min: 0,
    step: 1,
    disabled: false,
    hint: '',
    blankWhenZero: false,
    allowDecimal: false
  }
)

const emit = defineEmits<{
  save: [value: number]
}>()

const { t } = useI18n()
const displayValue = computed(() => {
  if (props.blankWhenZero && (props.modelValue == null || props.modelValue === 0)) {
    return '—'
  }
  return props.modelValue
})

const editing = ref(false)
const draft = ref(String(props.modelValue ?? 0))
const inputRef = ref<HTMLInputElement | null>(null)
let committing = false

watch(
  () => props.modelValue,
  (v) => {
    if (!editing.value) {
      draft.value = String(v ?? 0)
    }
  }
)

function startEdit() {
  if (props.disabled) return
  draft.value =
    props.blankWhenZero && (props.modelValue == null || props.modelValue === 0)
      ? ''
      : String(props.modelValue ?? 0)
  editing.value = true
  nextTick(() => {
    inputRef.value?.focus()
    inputRef.value?.select()
  })
}

function cancel() {
  editing.value = false
  draft.value = String(props.modelValue ?? 0)
}

function commit() {
  if (!editing.value || committing) return
  committing = true
  const parsed = Number(draft.value)
  if (!Number.isFinite(parsed)) {
    cancel()
    committing = false
    return
  }
  const next = Math.max(props.min, props.allowDecimal ? parsed : Math.trunc(parsed))
  editing.value = false
  if (next !== props.modelValue) {
    emit('save', next)
  }
  // blur after enter may fire twice; release on next tick
  nextTick(() => {
    committing = false
  })
}
</script>
