<template>
  <div class="card p-6">
    <!-- Toolbar: left filters (multi-line) + right actions -->
    <div class="flex flex-wrap items-end justify-between gap-4">
      <!-- Left: filters (allowed to wrap to multiple rows) -->
      <div class="flex flex-1 flex-wrap items-end gap-4">
        <!-- User Search -->
        <div ref="userSearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
          <label class="input-label">{{ t('admin.usage.userFilter') }}</label>
          <input
            v-model="userKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            autocomplete="off"
            @input="onUserInput"
            @focus="onUserFocus"
            @keydown.down.prevent="moveUserHighlight(1)"
            @keydown.up.prevent="moveUserHighlight(-1)"
            @keydown.enter.prevent="confirmUserHighlight"
            @keydown.esc="showUserDropdown = false"
          />
          <button
            v-if="filters.user_id"
            type="button"
            @click="clearUser"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear user filter"
          >
            ✕
          </button>
          <Teleport to="body">
            <div
              v-if="showUserDropdown"
              ref="userDropdownRef"
              class="usage-filter-portal-dropdown"
              :style="userDropdownStyle"
              @mousedown.prevent
            >
              <div v-if="userLoading" class="px-4 py-3 text-sm text-gray-500">
                {{ t('common.loading') }}
              </div>
              <template v-else>
                <div v-if="userRecentItems.length > 0 && !userKeyword.trim()" class="py-1">
                  <div class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
                    {{ t('admin.usage.recentPicks') }}
                  </div>
                  <button
                    v-for="(u, idx) in userRecentItems"
                    :key="'ru-' + u.id"
                    type="button"
                    class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                    :class="userHighlightIndex === idx && 'bg-gray-100 dark:bg-gray-700'"
                    @mouseenter="userHighlightIndex = idx"
                    @click="selectUser(u)"
                  >
                    <span>{{ u.email }}</span>
                    <span class="ml-2 text-xs text-gray-400">#{{ u.id }}</span>
                  </button>
                </div>
                <div v-if="userBrowseItems.length > 0" class="py-1" :class="userRecentItems.length > 0 && !userKeyword.trim() && 'border-t border-gray-100 dark:border-gray-700'">
                  <div
                    v-if="!userKeyword.trim()"
                    class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500"
                  >
                    {{ t('admin.usage.browseUsers') }}
                  </div>
                  <div
                    v-else
                    class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500"
                  >
                    {{ t('admin.usage.searchResults') }}
                  </div>
                  <button
                    v-for="(u, idx) in userBrowseItems"
                    :key="'bu-' + u.id"
                    type="button"
                    class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                    :class="userHighlightIndex === userRecentOffset + idx && 'bg-gray-100 dark:bg-gray-700'"
                    @mouseenter="userHighlightIndex = userRecentOffset + idx"
                    @click="selectUser(u)"
                  >
                    <span>{{ u.email }}</span>
                    <span class="ml-2 text-xs text-gray-400">#{{ u.id }}</span>
                  </button>
                </div>
                <div
                  v-if="!userLoading && userRecentItems.length === 0 && userBrowseItems.length === 0"
                  class="px-4 py-3 text-sm text-gray-500"
                >
                  {{ userKeyword.trim() ? t('common.noOptionsFound') : t('admin.usage.noRecentUsers') }}
                </div>
              </template>
            </div>
          </Teleport>
        </div>

        <!-- API Key Search -->
        <div ref="apiKeySearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
          <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
          <input
            v-model="apiKeyKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
            autocomplete="off"
            @input="debounceApiKeySearch"
            @focus="onApiKeyFocus"
          />
          <button
            v-if="filters.api_key_id"
            type="button"
            @click="onClearApiKey"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear API key filter"
          >
            ✕
          </button>
          <Teleport to="body">
            <div
              v-if="showApiKeyDropdown && apiKeyResults.length > 0"
              ref="apiKeyDropdownRef"
              class="usage-filter-portal-dropdown"
              :style="apiKeyDropdownStyle"
              @mousedown.prevent
            >
              <button
                v-for="k in apiKeyResults"
                :key="k.id"
                type="button"
                @click="selectApiKey(k)"
                class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
              >
                <span class="truncate">{{ k.name || `#${k.id}` }}</span>
                <span class="ml-2 text-xs text-gray-400">#{{ k.id }}</span>
              </button>
            </div>
          </Teleport>
        </div>

        <!-- Model Filter -->
        <div class="w-full sm:w-auto sm:min-w-[220px]">
          <label class="input-label">{{ t('usage.model') }}</label>
          <Select v-model="filters.model" :options="modelOptions" searchable @change="emitChange" />
        </div>

        <!-- Account Filter -->
        <div ref="accountSearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[220px]">
          <label class="input-label">{{ t('admin.usage.account') }}</label>
          <input
            v-model="accountKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchAccountPlaceholder')"
            autocomplete="off"
            @input="onAccountInput"
            @focus="onAccountFocus"
            @keydown.down.prevent="moveAccountHighlight(1)"
            @keydown.up.prevent="moveAccountHighlight(-1)"
            @keydown.enter.prevent="confirmAccountHighlight"
            @keydown.esc="showAccountDropdown = false"
          />
          <button
            v-if="filters.account_id"
            type="button"
            @click="clearAccount"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear account filter"
          >
            ✕
          </button>
          <Teleport to="body">
            <div
              v-if="showAccountDropdown"
              ref="accountDropdownRef"
              class="usage-filter-portal-dropdown"
              :style="accountDropdownStyle"
              @mousedown.prevent
            >
              <div v-if="accountLoading" class="px-4 py-3 text-sm text-gray-500">
                {{ t('common.loading') }}
              </div>
              <template v-else>
                <div v-if="accountRecentItems.length > 0 && !accountKeyword.trim()" class="py-1">
                  <div class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500">
                    {{ t('admin.usage.recentPicks') }}
                  </div>
                  <button
                    v-for="(a, idx) in accountRecentItems"
                    :key="'ra-' + a.id"
                    type="button"
                    class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                    :class="accountHighlightIndex === idx && 'bg-gray-100 dark:bg-gray-700'"
                    @mouseenter="accountHighlightIndex = idx"
                    @click="selectAccount(a)"
                  >
                    <span class="truncate">{{ a.name }}</span>
                    <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
                  </button>
                </div>
                <div
                  v-if="accountBrowseItems.length > 0"
                  class="py-1"
                  :class="accountRecentItems.length > 0 && !accountKeyword.trim() && 'border-t border-gray-100 dark:border-gray-700'"
                >
                  <div
                    v-if="!accountKeyword.trim()"
                    class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500"
                  >
                    {{ t('admin.usage.browseAccounts') }}
                  </div>
                  <div
                    v-else
                    class="px-3 py-1.5 text-xs font-medium text-gray-400 dark:text-gray-500"
                  >
                    {{ t('admin.usage.searchResults') }}
                  </div>
                  <button
                    v-for="(a, idx) in accountBrowseItems"
                    :key="'ba-' + a.id"
                    type="button"
                    class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700"
                    :class="accountHighlightIndex === accountRecentOffset + idx && 'bg-gray-100 dark:bg-gray-700'"
                    @mouseenter="accountHighlightIndex = accountRecentOffset + idx"
                    @click="selectAccount(a)"
                  >
                    <span class="truncate">{{ a.name }}</span>
                    <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
                  </button>
                </div>
                <div
                  v-if="!accountLoading && accountRecentItems.length === 0 && accountBrowseItems.length === 0"
                  class="px-4 py-3 text-sm text-gray-500"
                >
                  {{ accountKeyword.trim() ? t('common.noOptionsFound') : t('admin.usage.noRecentAccounts') }}
                </div>
              </template>
            </div>
          </Teleport>
        </div>

        <!-- Request Type Filter -->
        <div class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('usage.type') }}</label>
          <Select v-model="filters.request_type" :options="requestTypeOptions" @change="emitChange" />
        </div>

        <!-- Billing Type Filter -->
        <div class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.billingType') }}</label>
          <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="emitChange" />
        </div>

        <!-- Billing Mode Filter -->
        <div class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
          <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="emitChange" />
        </div>

        <!-- Group Filter -->
        <div class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.group') }}</label>
          <Select v-model="filters.group_id" :options="groupOptions" searchable @change="emitChange" />
        </div>
      </div>

      <!-- Right: actions -->
      <div v-if="showActions" class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
        <button type="button" @click="$emit('refresh')" class="btn btn-secondary">
          {{ t('common.refresh') }}
        </button>
        <button type="button" @click="$emit('reset')" class="btn btn-secondary">
          {{ t('common.reset') }}
        </button>
        <slot name="after-reset" />
        <button type="button" @click="$emit('cleanup')" class="btn btn-danger">
          {{ t('admin.usage.cleanup.button') }}
        </button>
        <button type="button" @click="$emit('export')" :disabled="exporting" class="btn btn-primary">
          {{ t('usage.exportExcel') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, toRef, watch, nextTick, type CSSProperties } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'
import { loadRecentPicks, pushRecentPick } from '@/composables/useRecentPicks'

type ModelValue = Record<string, any>

interface Props {
  modelValue: ModelValue
  exporting: boolean
  startDate: string
  endDate: string
  showActions?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true
})
const emit = defineEmits([
  'update:modelValue',
  'change',
  'refresh',
  'reset',
  'export',
  'cleanup'
])

const { t } = useI18n()
const route = useRoute()
const filters = toRef(props, 'modelValue')

const RECENT_USERS_KEY = 'admin-usage-recent-users'
const RECENT_ACCOUNTS_KEY = 'admin-usage-recent-accounts'
const BROWSE_LIMIT = 20

const userSearchRef = ref<HTMLElement | null>(null)
const apiKeySearchRef = ref<HTMLElement | null>(null)
const accountSearchRef = ref<HTMLElement | null>(null)
const userDropdownRef = ref<HTMLElement | null>(null)
const apiKeyDropdownRef = ref<HTMLElement | null>(null)
const accountDropdownRef = ref<HTMLElement | null>(null)

const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const userRecents = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
const userLoading = ref(false)
const userHighlightIndex = ref(-1)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null
let userSearchSeq = 0

const apiKeyKeyword = ref('')
const apiKeyResults = ref<SimpleApiKey[]>([])
const showApiKeyDropdown = ref(false)
let apiKeySearchTimeout: ReturnType<typeof setTimeout> | null = null

interface SimpleAccount {
  id: number
  name: string
}
const accountKeyword = ref('')
const accountResults = ref<SimpleAccount[]>([])
const accountRecents = ref<SimpleAccount[]>([])
const showAccountDropdown = ref(false)
const accountLoading = ref(false)
const accountHighlightIndex = ref(-1)
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null
let accountSearchSeq = 0

const userDropdownStyle = ref<CSSProperties>({})
const apiKeyDropdownStyle = ref<CSSProperties>({})
const accountDropdownStyle = ref<CSSProperties>({})

const modelOptions = ref<SelectOption[]>([{ value: null, label: t('admin.usage.allModels') }])
const groupOptions = ref<SelectOption[]>([{ value: null, label: t('admin.usage.allGroups') }])

const requestTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
  { value: 'cyber', label: t('usage.cyber') }
])

const billingTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') }
])

const billingModeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') }
])

const emitChange = () => emit('change')

const userRecentItems = computed(() => {
  if (userKeyword.value.trim()) return []
  return userRecents.value
})

const userBrowseItems = computed(() => {
  const recentIds = new Set(userRecentItems.value.map((u) => u.id))
  // When searching, show search hits; when browsing, exclude already-shown recents
  return userResults.value.filter((u) => !recentIds.has(u.id) || !!userKeyword.value.trim())
})

const userRecentOffset = computed(() => userRecentItems.value.length)

const userFlatOptions = computed(() => [...userRecentItems.value, ...userBrowseItems.value])

const accountRecentItems = computed(() => {
  if (accountKeyword.value.trim()) return []
  return accountRecents.value
})

const accountBrowseItems = computed(() => {
  const recentIds = new Set(accountRecentItems.value.map((a) => a.id))
  return accountResults.value.filter((a) => !recentIds.has(a.id) || !!accountKeyword.value.trim())
})

const accountRecentOffset = computed(() => accountRecentItems.value.length)

const accountFlatOptions = computed(() => [...accountRecentItems.value, ...accountBrowseItems.value])

function positionDropdown(anchor: HTMLElement | null, styleRef: typeof userDropdownStyle) {
  if (!anchor) return
  const input = anchor.querySelector('input') as HTMLElement | null
  const rect = (input || anchor).getBoundingClientRect()
  const width = Math.max(rect.width, 240)
  const maxHeight = 280
  const spaceBelow = window.innerHeight - rect.bottom
  const openUp = spaceBelow < maxHeight && rect.top > spaceBelow
  styleRef.value = {
    position: 'fixed',
    left: `${Math.max(8, rect.left)}px`,
    width: `${width}px`,
    zIndex: 9999,
    maxHeight: `${maxHeight}px`,
    ...(openUp
      ? { bottom: `${window.innerHeight - rect.top + 4}px`, top: 'auto' }
      : { top: `${rect.bottom + 4}px`, bottom: 'auto' })
  }
}

function updateAllDropdownPositions() {
  if (showUserDropdown.value) positionDropdown(userSearchRef.value, userDropdownStyle)
  if (showApiKeyDropdown.value) positionDropdown(apiKeySearchRef.value, apiKeyDropdownStyle)
  if (showAccountDropdown.value) positionDropdown(accountSearchRef.value, accountDropdownStyle)
}

function loadUserRecents() {
  userRecents.value = loadRecentPicks(RECENT_USERS_KEY).map((p) => ({
    id: p.id,
    email: p.label,
    deleted: false
  }))
}

function loadAccountRecents() {
  accountRecents.value = loadRecentPicks(RECENT_ACCOUNTS_KEY).map((p) => ({
    id: p.id,
    name: p.label
  }))
}

async function fetchBrowseUsers() {
  const seq = ++userSearchSeq
  userLoading.value = true
  try {
    const res = await adminAPI.users.list(1, BROWSE_LIMIT, {
      sort_by: 'last_active_at',
      sort_order: 'desc'
    })
    if (seq !== userSearchSeq) return
    userResults.value = (res.items || []).map((u) => ({
      id: u.id,
      email: u.email,
      deleted: !!u.deleted_at
    }))
  } catch {
    if (seq !== userSearchSeq) return
    userResults.value = []
  } finally {
    if (seq === userSearchSeq) userLoading.value = false
  }
}

async function fetchSearchUsers(keyword: string) {
  const seq = ++userSearchSeq
  userLoading.value = true
  try {
    const list = await adminAPI.usage.searchUsers(keyword)
    if (seq !== userSearchSeq) return
    userResults.value = list || []
  } catch {
    if (seq !== userSearchSeq) return
    userResults.value = []
  } finally {
    if (seq === userSearchSeq) userLoading.value = false
  }
}

async function fetchBrowseAccounts() {
  const seq = ++accountSearchSeq
  accountLoading.value = true
  try {
    const res = await adminAPI.accounts.list(1, BROWSE_LIMIT, {
      sort_by: 'last_used_at',
      sort_order: 'desc'
    })
    if (seq !== accountSearchSeq) return
    accountResults.value = (res.items || []).map((a) => ({ id: a.id, name: a.name }))
  } catch {
    if (seq !== accountSearchSeq) return
    accountResults.value = []
  } finally {
    if (seq === accountSearchSeq) accountLoading.value = false
  }
}

async function fetchSearchAccounts(keyword: string) {
  const seq = ++accountSearchSeq
  accountLoading.value = true
  try {
    const res = await adminAPI.accounts.list(1, BROWSE_LIMIT, { search: keyword })
    if (seq !== accountSearchSeq) return
    accountResults.value = (res.items || []).map((a) => ({ id: a.id, name: a.name }))
  } catch {
    if (seq !== accountSearchSeq) return
    accountResults.value = []
  } finally {
    if (seq === accountSearchSeq) accountLoading.value = false
  }
}

const onUserFocus = async () => {
  showUserDropdown.value = true
  loadUserRecents()
  userHighlightIndex.value = -1
  await nextTick()
  positionDropdown(userSearchRef.value, userDropdownStyle)
  if (!userKeyword.value.trim()) {
    // Always refresh browse list on empty focus so list is not search-dependent
    void fetchBrowseUsers()
  } else if (userResults.value.length === 0) {
    void fetchSearchUsers(userKeyword.value.trim())
  }
}

const onUserInput = () => {
  if (userSearchTimeout) clearTimeout(userSearchTimeout)
  showUserDropdown.value = true
  userHighlightIndex.value = -1
  void nextTick(() => positionDropdown(userSearchRef.value, userDropdownStyle))

  const kw = userKeyword.value.trim()
  // If user edits away from selected email, clear filter
  if (filters.value.user_id && kw === '') {
    // keep dropdown open for browse
  }

  userSearchTimeout = setTimeout(() => {
    if (!kw) {
      void fetchBrowseUsers()
      return
    }
    void fetchSearchUsers(kw)
  }, 250)
}

const moveUserHighlight = (delta: number) => {
  const opts = userFlatOptions.value
  if (!showUserDropdown.value || opts.length === 0) {
    void onUserFocus()
    return
  }
  const n = opts.length
  if (userHighlightIndex.value < 0) {
    userHighlightIndex.value = delta > 0 ? 0 : n - 1
  } else {
    userHighlightIndex.value = (userHighlightIndex.value + delta + n) % n
  }
}

const confirmUserHighlight = () => {
  const opts = userFlatOptions.value
  if (userHighlightIndex.value >= 0 && userHighlightIndex.value < opts.length) {
    selectUser(opts[userHighlightIndex.value])
  }
}

const selectUser = async (u: SimpleUser) => {
  userKeyword.value = u.email
  showUserDropdown.value = false
  filters.value.user_id = u.id
  pushRecentPick(RECENT_USERS_KEY, { id: u.id, label: u.email })
  loadUserRecents()
  clearApiKey()

  try {
    apiKeyResults.value = await adminAPI.usage.searchApiKeys(u.id, '')
  } catch {
    apiKeyResults.value = []
  }

  emitChange()
}

const clearUser = () => {
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  filters.value.user_id = undefined
  clearApiKey()
  emitChange()
}

const debounceApiKeySearch = () => {
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  apiKeySearchTimeout = setTimeout(async () => {
    try {
      apiKeyResults.value = await adminAPI.usage.searchApiKeys(
        filters.value.user_id,
        apiKeyKeyword.value || ''
      )
    } catch {
      apiKeyResults.value = []
    }
    await nextTick()
    positionDropdown(apiKeySearchRef.value, apiKeyDropdownStyle)
  }, 300)
}

const selectApiKey = (k: SimpleApiKey) => {
  apiKeyKeyword.value = k.name || String(k.id)
  showApiKeyDropdown.value = false
  filters.value.api_key_id = k.id
  emitChange()
}

const clearApiKey = () => {
  apiKeyKeyword.value = ''
  apiKeyResults.value = []
  showApiKeyDropdown.value = false
  filters.value.api_key_id = undefined
}

const onClearApiKey = () => {
  clearApiKey()
  emitChange()
}

const onApiKeyFocus = () => {
  showApiKeyDropdown.value = true
  void nextTick(() => positionDropdown(apiKeySearchRef.value, apiKeyDropdownStyle))
  if (apiKeyResults.value.length === 0) {
    debounceApiKeySearch()
  }
}

const onAccountFocus = async () => {
  showAccountDropdown.value = true
  loadAccountRecents()
  accountHighlightIndex.value = -1
  await nextTick()
  positionDropdown(accountSearchRef.value, accountDropdownStyle)
  if (!accountKeyword.value.trim()) {
    void fetchBrowseAccounts()
  } else if (accountResults.value.length === 0) {
    void fetchSearchAccounts(accountKeyword.value.trim())
  }
}

const onAccountInput = () => {
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
  showAccountDropdown.value = true
  accountHighlightIndex.value = -1
  void nextTick(() => positionDropdown(accountSearchRef.value, accountDropdownStyle))

  accountSearchTimeout = setTimeout(() => {
    const kw = accountKeyword.value.trim()
    if (!kw) {
      void fetchBrowseAccounts()
      return
    }
    void fetchSearchAccounts(kw)
  }, 250)
}

const moveAccountHighlight = (delta: number) => {
  const opts = accountFlatOptions.value
  if (!showAccountDropdown.value || opts.length === 0) {
    void onAccountFocus()
    return
  }
  const n = opts.length
  if (accountHighlightIndex.value < 0) {
    accountHighlightIndex.value = delta > 0 ? 0 : n - 1
  } else {
    accountHighlightIndex.value = (accountHighlightIndex.value + delta + n) % n
  }
}

const confirmAccountHighlight = () => {
  const opts = accountFlatOptions.value
  if (accountHighlightIndex.value >= 0 && accountHighlightIndex.value < opts.length) {
    selectAccount(opts[accountHighlightIndex.value])
  }
}

const selectAccount = (a: SimpleAccount) => {
  accountKeyword.value = a.name
  showAccountDropdown.value = false
  filters.value.account_id = a.id
  pushRecentPick(RECENT_ACCOUNTS_KEY, { id: a.id, label: a.name })
  loadAccountRecents()
  emitChange()
}

const clearAccount = () => {
  accountKeyword.value = ''
  accountResults.value = []
  showAccountDropdown.value = false
  filters.value.account_id = undefined
  emitChange()
}

const onDocumentClick = (e: MouseEvent) => {
  const target = e.target as Node | null
  if (!target) return

  const insideUser =
    (userSearchRef.value?.contains(target) ?? false) ||
    (userDropdownRef.value?.contains(target) ?? false)
  const insideApiKey =
    (apiKeySearchRef.value?.contains(target) ?? false) ||
    (apiKeyDropdownRef.value?.contains(target) ?? false)
  const insideAccount =
    (accountSearchRef.value?.contains(target) ?? false) ||
    (accountDropdownRef.value?.contains(target) ?? false)

  if (!insideUser) showUserDropdown.value = false
  if (!insideApiKey) showApiKeyDropdown.value = false
  if (!insideAccount) showAccountDropdown.value = false
}

watch(
  () => props.startDate,
  (value) => {
    filters.value.start_date = value
  },
  { immediate: true }
)

watch(
  () => props.endDate,
  (value) => {
    filters.value.end_date = value
  },
  { immediate: true }
)

watch(
  () => filters.value.user_id,
  (userId) => {
    if (!userId) {
      userKeyword.value = ''
      userResults.value = []
    }
  }
)

watch(
  () => filters.value.api_key_id,
  (apiKeyId) => {
    if (!apiKeyId) {
      apiKeyKeyword.value = ''
      apiKeyResults.value = []
    }
  }
)

async function resolveAccountLabel(accountId: number | string | undefined | null) {
  if (!accountId) {
    accountKeyword.value = ''
    accountResults.value = []
    return
  }
  const queryName = route.query.account_name
  const nameFromQuery = Array.isArray(queryName)
    ? queryName.find((v): v is string => typeof v === 'string' && v.length > 0)
    : typeof queryName === 'string' && queryName.length > 0
      ? queryName
      : undefined
  if (nameFromQuery) {
    accountKeyword.value = nameFromQuery
    return
  }
  if (accountKeyword.value) return
  // Prefer recent label
  const recent = loadRecentPicks(RECENT_ACCOUNTS_KEY).find((p) => p.id === Number(accountId))
  if (recent) {
    accountKeyword.value = recent.label
    return
  }
  try {
    const acc = await adminAPI.accounts.getById(Number(accountId))
    accountKeyword.value = acc.name || `#${accountId}`
  } catch {
    accountKeyword.value = `#${accountId}`
  }
}

async function resolveUserLabel(userId: number | string | undefined | null) {
  if (!userId) {
    userKeyword.value = ''
    return
  }
  if (userKeyword.value) return
  const recent = loadRecentPicks(RECENT_USERS_KEY).find((p) => p.id === Number(userId))
  if (recent) {
    userKeyword.value = recent.label
    return
  }
  try {
    const u = await adminAPI.users.getById(Number(userId))
    userKeyword.value = u.email || `#${userId}`
  } catch {
    userKeyword.value = `#${userId}`
  }
}

watch(
  () => filters.value.account_id,
  (accountId) => {
    void resolveAccountLabel(accountId)
  },
  { immediate: true }
)

watch(
  () => filters.value.user_id,
  (userId) => {
    void resolveUserLabel(userId)
  },
  { immediate: true }
)

onMounted(async () => {
  document.addEventListener('click', onDocumentClick)
  window.addEventListener('scroll', updateAllDropdownPositions, { capture: true, passive: true })
  window.addEventListener('resize', updateAllDropdownPositions)
  loadUserRecents()
  loadAccountRecents()
  void resolveAccountLabel(filters.value.account_id)
  void resolveUserLabel(filters.value.user_id)

  try {
    const [gs, ms] = await Promise.all([
      adminAPI.groups.list(1, 1000),
      adminAPI.dashboard.getModelStats({ start_date: props.startDate, end_date: props.endDate })
    ])

    groupOptions.value.push(...gs.items.map((g: any) => ({ value: g.id, label: g.name })))

    const uniqueModels = new Set<string>()
    ms.models?.forEach((s: any) => {
      if (s.model) {
        uniqueModels.add(s.model)
      }
    })
    modelOptions.value.push(
      ...Array.from(uniqueModels)
        .sort()
        .map((m) => ({ value: m, label: m }))
    )
  } catch {
    // Ignore filter option loading errors (page still usable)
  }
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('scroll', updateAllDropdownPositions, { capture: true } as any)
  window.removeEventListener('resize', updateAllDropdownPositions)
  if (userSearchTimeout) clearTimeout(userSearchTimeout)
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
})
</script>

<style scoped>
:global(.usage-filter-portal-dropdown) {
  overflow: auto;
  border-radius: 0.5rem;
  border: 1px solid rgb(229 231 235);
  background: white;
  box-shadow:
    0 10px 15px -3px rgb(0 0 0 / 0.1),
    0 4px 6px -4px rgb(0 0 0 / 0.1);
}

:global(.dark .usage-filter-portal-dropdown) {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}
</style>
