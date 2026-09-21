<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400"
          >
            <Icon name="chart" size="sm" />
          </span>
          {{ t('admin.channelLoadtest.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.channelLoadtest.description') }}
        </p>
      </header>

      <div class="grid gap-6 xl:grid-cols-[minmax(280px,380px)_1fr]">
        <section class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-4 inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 dark:border-dark-700 dark:bg-dark-900">
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium"
              :class="source === 'account' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-600 dark:text-dark-300'"
              @click="source = 'account'"
            >
              {{ t('admin.channelLoadtest.sourceAccount') }}
            </button>
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium"
              :class="source === 'manual' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-600 dark:text-dark-300'"
              @click="source = 'manual'"
            >
              {{ t('admin.channelLoadtest.sourceManual') }}
            </button>
          </div>

          <div v-if="source === 'account'" class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.account') }}
            </label>
            <input
              v-model="accountQuery"
              type="search"
              class="input"
              :placeholder="t('admin.channelLoadtest.accountPlaceholder')"
              @input="searchAccounts"
            />
            <select v-model.number="accountId" class="input">
              <option :value="0">{{ t('admin.channelLoadtest.accountPlaceholder') }}</option>
              <option v-for="acc in accounts" :key="acc.id" :value="acc.id">
                #{{ acc.id }} {{ acc.name }} ({{ acc.platform }}/{{ acc.type }})
              </option>
            </select>
            <p v-if="!accounts.length && accountQuery" class="text-xs text-gray-500">
              {{ t('admin.channelLoadtest.noAccounts') }}
            </p>
          </div>

          <div v-else class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.baseUrl') }}
            </label>
            <input v-model="baseUrl" class="input" placeholder="https://api.example.com" />
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.apiKey') }}
            </label>
            <input v-model="apiKey" class="input" type="password" autocomplete="off" />
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.apiKeyHint') }}</p>
          </div>

          <div class="mt-4 space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.proxy') }}
            </label>
            <select v-model.number="proxyId" class="input" :disabled="proxiesLoading">
              <option :value="0">{{ t('admin.channelLoadtest.noProxy') }}</option>
              <option v-for="proxy in proxies" :key="proxy.id" :value="proxy.id">
                {{ proxy.name }} ({{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }})
              </option>
            </select>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.proxyHint') }}</p>
          </div>

          <div class="mt-4 space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.model') }}
            </label>
            <input v-model="models" class="input" placeholder="kimi-k3, glm-5.3" />
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.modelHint') }}</p>

            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.profile') }}
            </label>
            <select v-model="profile" class="input">
              <option value="smoke">{{ t('admin.channelLoadtest.profileSmoke') }}</option>
              <option value="user363">{{ t('admin.channelLoadtest.profileUser363') }}</option>
              <option value="user363-stream">{{ t('admin.channelLoadtest.profileStream') }}</option>
              <option value="user363-sync">{{ t('admin.channelLoadtest.profileSync') }}</option>
              <option value="user363-sla">{{ t('admin.channelLoadtest.profileSla') }}</option>
            </select>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.apiMode') }}
                </label>
                <select v-model="apiMode" class="input">
                  <option value="chat_completions">{{ t('admin.channelLoadtest.apiModeCC') }}</option>
                  <option value="responses">{{ t('admin.channelLoadtest.apiModeRE') }}</option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.streamMode') }}
                </label>
                <select v-model="streamMode" class="input">
                  <option value="auto">{{ t('admin.channelLoadtest.streamModeAuto') }}</option>
                  <option value="stream">{{ t('admin.channelLoadtest.streamModeStream') }}</option>
                  <option value="sync">{{ t('admin.channelLoadtest.streamModeSync') }}</option>
                </select>
              </div>
            </div>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.streamModeHint') }}</p>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.concurrency') }}
                </label>
                <input v-model.number="concurrency" type="number" min="1" max="80" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.total') }}
                </label>
                <input v-model.number="total" type="number" min="1" max="500" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.maxTokens') }}
                </label>
                <input v-model.number="maxTokens" type="number" min="0" max="8000" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.sizeCap') }}
                </label>
                <input v-model.number="sizeCap" type="number" min="0" max="400000" class="input" />
              </div>
            </div>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.sizeCapHint') }}</p>

            <select v-model="tools" class="input">
              <option value="auto">{{ t('admin.channelLoadtest.toolsAuto') }}</option>
              <option value="off">{{ t('admin.channelLoadtest.toolsOff') }}</option>
              <option value="coding">{{ t('admin.channelLoadtest.toolsCoding') }}</option>
            </select>

            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="confirmCost" type="checkbox" />
              {{ t('admin.channelLoadtest.confirmCost') }}
            </label>
          </div>

          <div class="mt-5 flex gap-2">
            <button
              type="button"
              class="btn btn-primary flex-1"
              :disabled="busy || running"
              @click="startRun"
            >
              {{ t('admin.channelLoadtest.start') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="!running"
              @click="stopRun"
            >
              {{ t('admin.channelLoadtest.stop') }}
            </button>
          </div>
        </section>

        <section class="space-y-4">
          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ running ? t('admin.channelLoadtest.running') : t('admin.channelLoadtest.live') }}
              </h2>
              <span v-if="snap" class="text-xs text-gray-500">
                {{ snap.status }} · {{ snap.id?.slice(0, 8) }}
                <template v-if="snap.api_mode"> · {{ snap.api_mode }}</template>
                <template v-if="snap.stream_mode"> · {{ snap.stream_mode }}</template>
                <template v-if="snap.proxy_name"> · {{ snap.proxy_name }}</template>
              </span>
            </div>
            <div v-if="snap" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">in-flight</div>
                <div class="text-lg font-semibold">{{ snap.inflight }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.peak') }}</div>
                <div class="text-lg font-semibold">{{ snap.peak }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.done') }}</div>
                <div class="text-lg font-semibold">{{ snap.done }} / {{ snap.total }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.success') }}</div>
                <div class="text-lg font-semibold">{{ snap.success_rate.toFixed(1) }}%</div>
              </div>
            </div>
            <p v-else class="mt-3 text-sm text-gray-500">{{ t('admin.channelLoadtest.idle') }}</p>
            <p v-if="snap?.error" class="mt-3 text-sm text-red-600">{{ snap.error }}</p>
            <p
              v-if="snap && snap.model_missing_requests > 0"
              class="mt-3 text-sm text-amber-700 dark:text-amber-400"
            >
              {{ t('admin.channelLoadtest.emptyModelWarn', { n: snap.model_missing_requests }) }}
            </p>
          </div>

          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.channelLoadtest.slaTitle') }}
              </h2>
              <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-0.5 text-xs dark:border-dark-700 dark:bg-dark-900">
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 'ms' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 'ms'"
                >
                  {{ t('admin.channelLoadtest.timeUnitMs') }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 's' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 's'"
                >
                  {{ t('admin.channelLoadtest.timeUnitS') }}
                </button>
              </div>
            </div>
            <table v-if="snap?.sla?.length" class="mt-3 w-full text-left text-sm">
              <thead>
                <tr class="text-xs text-gray-500">
                  <th class="py-1">metric</th>
                  <th class="py-1">want</th>
                  <th class="py-1">got</th>
                  <th class="py-1"> </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in snap.sla" :key="row.name" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ row.name }}</td>
                  <td class="py-1.5 text-gray-500">{{ formatSlaWant(row) }}</td>
                  <td class="py-1.5">{{ formatSlaGot(row) }}</td>
                  <td class="py-1.5">
                    <span
                      class="rounded px-1.5 py-0.5 text-xs font-semibold"
                      :class="row.skip ? 'bg-gray-100 text-gray-500' : row.pass ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'"
                    >
                      {{ row.skip ? 'n/a' : row.pass ? t('admin.channelLoadtest.pass') : t('admin.channelLoadtest.fail') }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-2 text-sm text-gray-500">{{ t('admin.channelLoadtest.noResults') }}</p>
          </div>

          <div class="overflow-hidden rounded-3xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex items-center justify-between gap-3 px-5 py-3">
              <div class="text-sm font-semibold">
                {{ t('admin.channelLoadtest.results') }}
                <span v-if="snap?.results?.length" class="ml-1 font-normal text-gray-500">
                  ({{ snap.results.length }})
                </span>
              </div>
              <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-0.5 text-xs dark:border-dark-700 dark:bg-dark-900">
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 'ms' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 'ms'"
                >
                  {{ t('admin.channelLoadtest.timeUnitMs') }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 's' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 's'"
                >
                  {{ t('admin.channelLoadtest.timeUnitS') }}
                </button>
              </div>
            </div>
            <div class="max-h-[28rem] overflow-auto">
              <table class="min-w-full text-left text-xs">
                <thead class="sticky top-0 bg-gray-50 text-gray-500 dark:bg-dark-900">
                  <tr>
                    <th class="px-3 py-2">#</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.requestedModel') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.outcome') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.ttft') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.duration') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.tpot') }}</th>
                    <th class="px-3 py-2">model</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="row in snap?.results || []"
                    :key="row.seq"
                    class="border-t border-gray-100 dark:border-dark-700"
                  >
                    <td class="px-3 py-1.5">{{ row.seq }}</td>
                    <td class="px-3 py-1.5">{{ row.requested_model }}</td>
                    <td class="px-3 py-1.5">{{ row.outcome }}</td>
                    <td class="px-3 py-1.5">{{ formatMs(row.first_content_ms) }}</td>
                    <td class="px-3 py-1.5">{{ formatMs(row.duration_ms) }}</td>
                    <td class="px-3 py-1.5">{{ row.tpot_tok_s ? row.tpot_tok_s.toFixed(1) : '-' }}</td>
                    <td class="px-3 py-1.5">{{ row.response_model || (row.model_missing_chunks ? 'missing' : '-') }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { LoadtestAPIMode, LoadtestProfile, LoadtestSLAVerdict, LoadtestSnapshot, LoadtestStreamMode } from '@/api/admin/channelLoadtest'
import { list as listAccounts } from '@/api/admin/accounts'
import { getAll as listProxies } from '@/api/admin/proxies'
import type { Account, Proxy } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const source = ref<'account' | 'manual'>('account')
const accountQuery = ref('')
const accountId = ref(0)
const accounts = ref<Account[]>([])
const proxyId = ref(0)
const proxies = ref<Proxy[]>([])
const proxiesLoading = ref(false)
const baseUrl = ref('')
const apiKey = ref('')
const models = ref('kimi-k3')
const profile = ref<LoadtestProfile>('smoke')
const apiMode = ref<LoadtestAPIMode>('chat_completions')
const streamMode = ref<LoadtestStreamMode>('auto')
const concurrency = ref(20)
const total = ref(40)
const maxTokens = ref(256)
const sizeCap = ref(80000)
const tools = ref('auto')
const confirmCost = ref(false)
const busy = ref(false)
const snap = ref<LoadtestSnapshot | null>(null)
const timeUnit = ref<'ms' | 's'>((localStorage.getItem('channel-loadtest-time-unit') as 'ms' | 's') || 'ms')
let pollTimer: ReturnType<typeof setInterval> | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const running = computed(() => snap.value?.status === 'running' || snap.value?.status === 'stopping')

watch(timeUnit, (unit) => {
  localStorage.setItem('channel-loadtest-time-unit', unit)
})

function formatMs(ms?: number) {
  if (ms == null || ms <= 0) return '-'
  if (timeUnit.value === 's') {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

function formatSlaWant(row: LoadtestSLAVerdict) {
  if (row.metric === 'ttft' && row.want_value) {
    return `<${formatMs(row.want_value)}`
  }
  return row.want
}

function formatSlaGot(row: LoadtestSLAVerdict) {
  if (row.skip) return 'n/a'
  if (row.metric === 'ttft') {
    const n = row.samples ? ` (n=${row.samples})` : ''
    return `${formatMs(row.got_value)}${n}`
  }
  return row.got
}

async function loadProxies() {
  proxiesLoading.value = true
  try {
    proxies.value = await listProxies()
  } catch {
    proxies.value = []
  } finally {
    proxiesLoading.value = false
  }
}

watch(accountId, (id) => {
  const acc = accounts.value.find((item) => item.id === id)
  proxyId.value = acc?.proxy_id && acc.proxy_id > 0 ? acc.proxy_id : 0
})

async function loadAccounts() {
  try {
    const res = await listAccounts(1, 30, {
      search: accountQuery.value || undefined,
      type: 'apikey',
      lite: '1'
    })
    accounts.value = res.items || []
  } catch {
    accounts.value = []
  }
}

function searchAccounts() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void loadAccounts()
  }, 250)
}

async function startRun() {
  busy.value = true
  try {
    const payload: Parameters<typeof adminAPI.channelLoadtest.start>[0] = {
      model: models.value.split(',')[0]?.trim(),
      models: models.value,
      profile: profile.value,
      api_mode: apiMode.value,
      stream_mode: streamMode.value,
      concurrency: concurrency.value,
      total: total.value,
      max_tokens: maxTokens.value,
      size_cap: sizeCap.value,
      tools: tools.value,
      confirm_cost: confirmCost.value
    }
    if (source.value === 'account') {
      if (!accountId.value) {
        appStore.showError(t('admin.channelLoadtest.account'))
        return
      }
      payload.account_id = accountId.value
    } else {
      payload.base_url = baseUrl.value
      payload.api_key = apiKey.value
    }
    payload.proxy_id = proxyId.value > 0 ? proxyId.value : 0
    snap.value = await adminAPI.channelLoadtest.start(payload)
    startPolling()
  } catch (err) {
    appStore.showError((err as Error).message || t('admin.channelLoadtest.startError'))
  } finally {
    busy.value = false
  }
}

async function stopRun() {
  if (!snap.value?.id) return
  try {
    snap.value = await adminAPI.channelLoadtest.stop(snap.value.id)
  } catch (err) {
    appStore.showError((err as Error).message || t('admin.channelLoadtest.stopError'))
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    if (!snap.value?.id) return
    try {
      snap.value = await adminAPI.channelLoadtest.get(snap.value.id)
      if (!running.value) stopPolling()
    } catch {
      appStore.showError(t('admin.channelLoadtest.loadError'))
    }
  }, 500)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(async () => {
  await Promise.all([loadAccounts(), loadProxies()])
  try {
    const latest = await adminAPI.channelLoadtest.latest()
    if (latest) {
      snap.value = latest
      if (latest.status === 'running' || latest.status === 'stopping') startPolling()
    }
  } catch {
    /* ignore */
  }
})

onUnmounted(() => {
  stopPolling()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>
