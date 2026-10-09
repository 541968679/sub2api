<template>
  <div
    class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600"
    data-testid="newapi-wallet-fields"
  >
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.title') }}</label>
      <p class="input-hint">{{ t('admin.accounts.newapiWallet.hint') }}</p>
    </div>
    <div v-if="showSourcePicker">
      <label class="input-label" for="newapi-wallet-source">
        {{ t('admin.accounts.newapiWallet.reuse') }}
      </label>
      <select
        id="newapi-wallet-source"
        class="input"
        data-testid="newapi-wallet-source"
        :value="sourceValue"
        @change="onSourceChange"
      >
        <option value="">{{ t('admin.accounts.newapiWallet.manual') }}</option>
        <optgroup
          v-if="decision.sameOrigin.length"
          :label="t('admin.accounts.newapiWallet.sameOriginGroup')"
        >
          <option v-for="row in decision.sameOrigin" :key="row.id" :value="String(row.id)">
            {{ sourceLabel(row) }}
          </option>
        </optgroup>
        <optgroup v-if="decision.others.length" :label="t('admin.accounts.newapiWallet.otherGroup')">
          <option v-for="row in decision.others" :key="row.id" :value="String(row.id)">
            {{ sourceLabel(row) }}
          </option>
        </optgroup>
      </select>
      <p
        v-if="hint"
        class="input-hint"
        data-testid="newapi-wallet-source-hint"
        :data-match="hint.state"
      >
        {{ t(hint.key, hint.params) }}
      </p>
    </div>
    <p
      v-else-if="loadFailed"
      class="input-hint"
      data-testid="newapi-wallet-source-hint"
      data-match="error"
    >
      {{ t('admin.accounts.newapiWallet.loadFailed') }}
    </p>
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.userId') }}</label>
      <input
        :value="userId"
        type="text"
        inputmode="numeric"
        class="input font-mono"
        data-testid="newapi-wallet-user-id"
        :placeholder="t('admin.accounts.newapiWallet.userIdPlaceholder')"
        @input="onUserIdInput"
      />
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.accessToken') }}</label>
      <input
        :value="accessToken"
        type="password"
        class="input font-mono"
        autocomplete="new-password"
        data-1p-ignore
        data-lpignore="true"
        data-bwignore="true"
        data-testid="newapi-wallet-access-token"
        :placeholder="tokenPlaceholder"
        @input="onTokenInput"
      />
    </div>
    <button
      v-if="hasSavedToken || userId || accessToken || sourceAccountId"
      type="button"
      class="text-sm text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400"
      data-testid="newapi-wallet-clear"
      @click="onClear"
    >
      {{ t('admin.accounts.newapiWallet.clear') }}
    </button>
    <p v-if="hasSavedToken || userId || accessToken || sourceAccountId" class="input-hint">
      {{ t('admin.accounts.newapiWallet.clearHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import {
  canonicalNewAPIWalletOrigin,
  decideNewAPIWalletReuse,
  type NewAPIWalletSource
} from '@/components/account/credentialsBuilder'

const props = defineProps<{
  userId: string
  accessToken: string
  hasSavedToken?: boolean
  baseUrl?: string
  excludeAccountId?: number | null
  sourceAccountId?: number | null
}>()

const emit = defineEmits<{
  'update:userId': [value: string]
  'update:accessToken': [value: string]
  'update:sourceAccountId': [value: number | null]
  clear: []
}>()

const { t } = useI18n()
const sources = ref<NewAPIWalletSource[]>([])
const loadFailed = ref(false)
const userLocked = ref(false)
let active = true

const decision = computed(() =>
  decideNewAPIWalletReuse(sources.value, props.baseUrl ?? '', props.excludeAccountId)
)

const showSourcePicker = computed(
  () => decision.value.sameOrigin.length + decision.value.others.length > 0
)

const sourceValue = computed(() =>
  props.sourceAccountId != null && props.sourceAccountId > 0 ? String(props.sourceAccountId) : ''
)

const selectedSource = computed(
  () => sources.value.find((row) => row.id === props.sourceAccountId) ?? null
)

const tokenPlaceholder = computed(() => {
  if (props.sourceAccountId && !props.accessToken.trim()) {
    return t('admin.accounts.newapiWallet.tokenFromSource')
  }
  return props.hasSavedToken
    ? t('admin.accounts.newapiWallet.accessTokenKeep')
    : t('admin.accounts.newapiWallet.accessTokenPlaceholder')
})

const hint = computed(() => {
  const selected = selectedSource.value
  if (selected) {
    const peers = decision.value.sameOrigin.filter((row) => row.user_id === selected.user_id).length - 1
    if (!userLocked.value && decision.value.auto?.id === selected.id) {
      if (peers > 0) {
        return {
          state: 'auto',
          key: 'admin.accounts.newapiWallet.matchedPeers',
          params: { name: selected.name, userId: selected.user_id, count: peers }
        }
      }
      return {
        state: 'auto',
        key: 'admin.accounts.newapiWallet.matched',
        params: { name: selected.name, userId: selected.user_id }
      }
    }
    return { state: 'manual', key: 'admin.accounts.newapiWallet.reuseHint', params: {} }
  }
  if (decision.value.distinctUserCount > 1) {
    return {
      state: 'ambiguous',
      key: 'admin.accounts.newapiWallet.ambiguous',
      params: { count: decision.value.distinctUserCount }
    }
  }
  return null
})

const sourceLabel = (row: NewAPIWalletSource) =>
  t('admin.accounts.newapiWallet.option', {
    name: row.name,
    origin: row.origin,
    userId: row.user_id
  })

const applySource = (row: NewAPIWalletSource) => {
  if (props.sourceAccountId !== row.id) emit('update:sourceAccountId', row.id)
  if (props.userId !== row.user_id) emit('update:userId', row.user_id)
}

const syncAuto = () => {
  if (userLocked.value || props.hasSavedToken || props.accessToken.trim()) return
  const sourceId = props.sourceAccountId ?? null
  const origin = canonicalNewAPIWalletOrigin(props.baseUrl ?? '')
  if (sourceId != null) {
    const selected = sources.value.find((row) => row.id === sourceId)
    if (selected && canonicalNewAPIWalletOrigin(selected.origin) === origin && origin !== '') {
      if (props.userId !== selected.user_id) emit('update:userId', selected.user_id)
      return
    }
    if (decision.value.auto) {
      applySource(decision.value.auto)
      return
    }
    emit('update:sourceAccountId', null)
    if (selected && props.userId === selected.user_id) emit('update:userId', '')
    return
  }
  if (props.userId.trim()) return
  if (decision.value.auto) applySource(decision.value.auto)
}

watch(
  () => ({
    autoId: decision.value.auto?.id ?? null,
    origin: canonicalNewAPIWalletOrigin(props.baseUrl ?? ''),
    hasSaved: !!props.hasSavedToken,
    userId: props.userId,
    token: props.accessToken,
    sourceId: props.sourceAccountId ?? null,
    locked: userLocked.value,
    loaded: sources.value.length
  }),
  () => {
    syncAuto()
  },
  { immediate: true }
)

watch(
  () => canonicalNewAPIWalletOrigin(props.baseUrl ?? ''),
  (next, prev) => {
    if (prev === undefined || next === prev) return
    if (
      !props.userId.trim() &&
      !props.accessToken.trim() &&
      !props.hasSavedToken &&
      (props.sourceAccountId == null || props.sourceAccountId <= 0)
    ) {
      userLocked.value = false
    }
  }
)

const onSourceChange = (event: Event) => {
  userLocked.value = true
  const value = (event.target as HTMLSelectElement).value
  if (!value) {
    if (props.sourceAccountId) emit('update:sourceAccountId', null)
    return
  }
  const id = Number(value)
  const row = sources.value.find((item) => item.id === id)
  if (!row) return
  applySource(row)
  if (props.accessToken.trim()) emit('update:accessToken', '')
}

const onUserIdInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  const selected = selectedSource.value
  if (!selected || selected.user_id !== value.trim()) {
    userLocked.value = true
    if (props.sourceAccountId) emit('update:sourceAccountId', null)
  }
  emit('update:userId', value)
}

const onTokenInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  if (value.trim()) {
    userLocked.value = true
    if (props.sourceAccountId) emit('update:sourceAccountId', null)
  }
  emit('update:accessToken', value)
}

const onClear = () => {
  userLocked.value = true
  if (props.sourceAccountId) emit('update:sourceAccountId', null)
  emit('clear')
}

onMounted(async () => {
  try {
    const rows = await adminAPI.accounts.listNewAPIWalletSources()
    if (!active) return
    sources.value = Array.isArray(rows) ? rows : []
  } catch {
    if (!active) return
    loadFailed.value = true
    sources.value = []
  }
})

onUnmounted(() => {
  active = false
})
</script>
