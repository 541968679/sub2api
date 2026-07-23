<template>
  <div class="card p-4">
    <div class="flex flex-wrap items-end gap-3">
      <div class="w-full sm:w-40">
        <label class="input-label">{{ t('usage.errors.filters.platform') }}</label>
        <Select v-model="local.platform" :options="platformOptions" @change="emitChange" />
      </div>
      <div class="w-full sm:w-36">
        <label class="input-label">{{ t('usage.errors.filters.bridge') }}</label>
        <Select v-model="local.bridge" :options="bridgeOptions" @change="emitChange" />
      </div>
      <div class="w-full sm:min-w-[160px] sm:w-48">
        <label class="input-label">{{ t('usage.errors.filters.userQuery') }}</label>
        <input
          v-model="local.user_query"
          type="text"
          class="input"
          :placeholder="t('usage.errors.filters.userQueryPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="w-full sm:min-w-[160px] sm:w-48">
        <label class="input-label">{{ t('usage.errors.filters.model') }}</label>
        <input
          v-model="local.model"
          type="text"
          class="input font-mono text-xs"
          :placeholder="t('usage.errors.filters.modelPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="w-full sm:min-w-[160px] sm:w-48">
        <label class="input-label">{{ t('usage.errors.filters.upstreamModel') }}</label>
        <input
          v-model="local.upstream_model"
          type="text"
          class="input font-mono text-xs"
          :placeholder="t('usage.errors.filters.upstreamModelPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="w-full sm:w-40">
        <label class="input-label">{{ t('usage.errors.filters.groupId') }}</label>
        <input
          v-model="local.group_id"
          type="text"
          class="input"
          :placeholder="t('usage.errors.filters.idPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="w-full sm:w-40">
        <label class="input-label">{{ t('usage.errors.filters.accountId') }}</label>
        <input
          v-model="local.account_id"
          type="text"
          class="input"
          :placeholder="t('usage.errors.filters.idPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="w-full sm:min-w-[180px] sm:w-56">
        <label class="input-label">{{ t('usage.errors.filters.q') }}</label>
        <input
          v-model="local.q"
          type="text"
          class="input"
          :placeholder="t('usage.errors.filters.qPlaceholder')"
          @change="emitChange"
          @keyup.enter="emitChange"
        />
      </div>
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('refresh')">
          {{ t('common.refresh') }}
        </button>
        <button type="button" class="btn btn-secondary" @click="onReset">
          {{ t('common.reset') }}
        </button>
      </div>
    </div>

    <div class="mt-3">
      <p class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('usage.errors.filters.statusCodes') }}
      </p>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="code in statusCodePresets"
          :key="code"
          type="button"
          class="rounded-full px-3 py-1 text-xs font-semibold ring-1 ring-inset transition-colors"
          :class="
            selectedCodes.has(code)
              ? 'bg-primary-600 text-white ring-primary-600'
              : 'bg-gray-50 text-gray-600 ring-gray-200 hover:bg-gray-100 dark:bg-dark-800 dark:text-gray-300 dark:ring-dark-600'
          "
          @click="toggleCode(code)"
        >
          {{ code }}
        </button>
      </div>
      <p class="mt-1 text-[11px] text-gray-400">
        {{ t('usage.errors.filters.statusCodesHint') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'

export type ErrorRequestFilterState = {
  platform: string
  bridge: string
  user_query: string
  model: string
  upstream_model: string
  group_id: string
  account_id: string
  q: string
  status_codes: number[]
}

const props = defineProps<{
  modelValue: ErrorRequestFilterState
}>()

const emit = defineEmits<{
  'update:modelValue': [ErrorRequestFilterState]
  change: []
  refresh: []
  reset: []
}>()

const { t } = useI18n()

const local = reactive<ErrorRequestFilterState>({
  platform: '',
  bridge: 'all',
  user_query: '',
  model: '',
  upstream_model: '',
  group_id: '',
  account_id: '',
  q: '',
  status_codes: []
})

watch(
  () => props.modelValue,
  (v) => {
    Object.assign(local, {
      platform: v.platform || '',
      bridge: v.bridge || 'all',
      user_query: v.user_query || '',
      model: v.model || '',
      upstream_model: v.upstream_model || '',
      group_id: v.group_id || '',
      account_id: v.account_id || '',
      q: v.q || '',
      status_codes: [...(v.status_codes || [])]
    })
  },
  { immediate: true, deep: true }
)

const platformOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'grok', label: 'Grok' }
])

const bridgeOptions = computed(() => [
  { value: 'all', label: t('usage.errors.filters.bridgeAll') },
  { value: 'bridge', label: t('usage.errors.filters.bridgeOnly') },
  { value: 'non_bridge', label: t('usage.errors.filters.bridgeNon') }
])

const statusCodePresets = [400, 401, 403, 429, 500, 502, 503, 504, 529]

const selectedCodes = computed(() => new Set(local.status_codes))

function emitChange() {
  emit('update:modelValue', {
    platform: local.platform,
    bridge: local.bridge,
    user_query: local.user_query,
    model: local.model,
    upstream_model: local.upstream_model,
    group_id: local.group_id,
    account_id: local.account_id,
    q: local.q,
    status_codes: [...local.status_codes]
  })
  emit('change')
}

function toggleCode(code: number) {
  const idx = local.status_codes.indexOf(code)
  if (idx >= 0) {
    local.status_codes.splice(idx, 1)
  } else {
    local.status_codes.push(code)
  }
  emitChange()
}

function onReset() {
  emit('reset')
}
</script>
