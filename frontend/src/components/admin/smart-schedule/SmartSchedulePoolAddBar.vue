<template>
  <section
    class="w-fit max-w-full rounded-lg border border-gray-200 bg-gray-50/80 px-1.5 py-1 dark:border-dark-600 dark:bg-dark-800/60"
    data-testid="smart-schedule-add-region"
    :title="t('admin.users.smartSchedule.addRegionHint')"
  >
    <div class="flex flex-wrap items-center gap-1.5">
      <span class="shrink-0 text-xs font-medium uppercase tracking-wide text-gray-400">
        {{ t('admin.users.smartSchedule.addRegionTitle') }}
      </span>
      <div class="flex flex-wrap items-center gap-1.5" data-testid="smart-schedule-add-cluster">
        <label class="flex w-56 max-w-full items-center gap-1.5">
          <span class="shrink-0 text-xs text-gray-500">{{ t('admin.users.smartSchedule.addAccount') }}</span>
          <div class="relative min-w-0 flex-1">
          <input
            :value="searchQuery"
            type="text"
            class="input min-w-0 w-full !px-2 !py-1"
            data-testid="smart-schedule-add-select"
            :placeholder="t('admin.users.smartSchedule.addAccountPlaceholder')"
            autocomplete="off"
            @focus="$emit('update:searchOpen', true)"
            @input="onSearchInput"
            @blur="$emit('search-blur')"
          />
          <div
            v-if="searchOpen"
            class="absolute left-0 right-0 top-full z-20 mt-1 max-h-60 overflow-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
            data-testid="smart-schedule-add-dropdown"
            @mousedown.prevent
          >
            <button
              v-for="account in filteredAccounts"
              :key="account.id"
              type="button"
              class="block w-full px-3 py-2 text-left text-sm text-gray-800 hover:bg-gray-100 dark:text-gray-100 dark:hover:bg-dark-700"
              :data-testid="`smart-schedule-add-option-${account.id}`"
              @click="$emit('choose', account.id)"
            >
              {{ account.name }} (#{{ account.id }})
            </button>
            <p v-if="filteredAccounts.length === 0" class="px-3 py-2 text-sm text-gray-500">
              {{ t('admin.users.smartSchedule.addAccountEmpty') }}
            </p>
          </div>
          </div>
        </label>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          data-testid="smart-schedule-filtered-add"
          @click="$emit('open-filtered-add')"
        >
          {{ t('admin.users.smartSchedule.filteredAdd') }}
        </button>
        <div class="flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-gray-500">{{ t('admin.users.smartSchedule.addScheduling') }}</span>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            data-testid="smart-schedule-add-api"
            @click="$emit('add-scheduling', 'apikey')"
          >
            {{ t('admin.users.smartSchedule.addSchedulingApi', { count: apiCount }) }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            data-testid="smart-schedule-add-oauth"
            @click="$emit('add-scheduling', 'oauth')"
          >
            {{ t('admin.users.smartSchedule.addSchedulingOauth', { count: oauthCount }) }}
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            data-testid="smart-schedule-add-all"
            @click="$emit('add-scheduling', 'all')"
          >
            {{ t('admin.users.smartSchedule.addSchedulingAll', { count: allCount }) }}
          </button>
        </div>
      </div>

      <div class="ml-auto flex flex-wrap items-center gap-1.5" data-testid="smart-schedule-add-ops">
        <button
          type="button"
          class="btn btn-secondary px-2 md:px-3"
          :disabled="refreshDisabled"
          :title="t('admin.users.smartSchedule.refresh')"
          data-testid="smart-schedule-refresh"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
          <span class="hidden md:inline">{{ t('admin.users.smartSchedule.refresh') }}</span>
        </button>
        <div class="relative" ref="autoRefreshDropdownRef">
          <button
            type="button"
            class="btn btn-secondary px-2 md:px-3"
            :title="t('admin.accounts.autoRefresh')"
            data-testid="smart-schedule-auto-refresh"
            @click="showAutoRefreshDropdown = !showAutoRefreshDropdown"
          >
            <span
              class="inline-flex"
              :class="autoRefreshEnabled ? 'animate-spin' : ''"
              data-testid="smart-schedule-auto-refresh-icon"
            >
              <Icon name="refresh" size="sm" />
            </span>
            <span class="hidden md:inline">
              {{
                autoRefreshEnabled
                  ? t('admin.accounts.autoRefreshCountdown', { seconds: autoRefreshCountdown })
                  : t('admin.accounts.autoRefresh')
              }}
            </span>
          </button>
          <div
            v-if="showAutoRefreshDropdown"
            class="absolute right-0 z-50 mt-2 w-56 origin-top-right rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
            data-testid="smart-schedule-auto-refresh-menu"
          >
            <div class="p-2">
              <button
                type="button"
                class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                @click="$emit('set-auto-refresh-enabled', !autoRefreshEnabled)"
              >
                <span>{{ t('admin.accounts.enableAutoRefresh') }}</span>
                <Icon v-if="autoRefreshEnabled" name="check" size="sm" class="text-primary-500" />
              </button>
              <div class="my-1 border-t border-gray-100 dark:border-gray-700"></div>
              <button
                v-for="sec in autoRefreshIntervals"
                :key="sec"
                type="button"
                class="flex w-full items-center justify-between rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                @click="$emit('set-auto-refresh-interval', sec)"
              >
                <span>{{ autoRefreshIntervalLabel(sec) }}</span>
                <Icon v-if="autoRefreshIntervalSeconds === sec" name="check" size="sm" class="text-primary-500" />
              </button>
            </div>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="poolEmpty || autoSorting"
          :title="t('admin.users.smartSchedule.autoSortHint')"
          data-testid="smart-schedule-auto-sort"
          @click="$emit('auto-sort')"
        >
          {{
            autoSorting
              ? t('admin.users.smartSchedule.autoSortProgress', { done: autoSortDone, total: autoSortTotal })
              : t('admin.users.smartSchedule.autoSort')
          }}
        </button>
        <HelpTooltip :content="t('admin.users.smartSchedule.autoSortHint')" width-class="w-80" />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Account } from '@/types'
import type { SmartScheduleAddScope } from '@/composables/useUserSmartScheduleEditor'

type SmartScheduleAutoRefreshInterval = 5 | 10 | 15 | 30

defineProps<{
  searchQuery: string
  searchOpen: boolean
  filteredAccounts: Account[]
  apiCount: number
  oauthCount: number
  allCount: number
  candidatesReady?: boolean
  poolEmpty: boolean
  autoSorting: boolean
  autoSortDone: number
  autoSortTotal: number
  refreshing: boolean
  refreshDisabled: boolean
  autoRefreshEnabled: boolean
  autoRefreshCountdown: number
  autoRefreshIntervalSeconds: SmartScheduleAutoRefreshInterval
  autoRefreshIntervals: readonly SmartScheduleAutoRefreshInterval[]
}>()

const emit = defineEmits<{
  'update:searchQuery': [value: string]
  'update:searchOpen': [value: boolean]
  'search-blur': []
  choose: [accountId: number]
  'open-filtered-add': []
  'add-scheduling': [scope: SmartScheduleAddScope]
  'auto-sort': []
  refresh: []
  'set-auto-refresh-enabled': [enabled: boolean]
  'set-auto-refresh-interval': [seconds: SmartScheduleAutoRefreshInterval]
}>()

const { t } = useI18n()
const showAutoRefreshDropdown = ref(false)
const autoRefreshDropdownRef = ref<HTMLElement | null>(null)

function autoRefreshIntervalLabel(sec: number) {
  if (sec === 5) return t('admin.accounts.refreshInterval5s')
  if (sec === 10) return t('admin.accounts.refreshInterval10s')
  if (sec === 15) return t('admin.accounts.refreshInterval15s')
  return t('admin.accounts.refreshInterval30s')
}

function onSearchInput(event: Event) {
  emit('update:searchQuery', (event.target as HTMLInputElement).value)
  emit('update:searchOpen', true)
}

function handleAutoRefreshClickOutside(event: MouseEvent) {
  const target = event.target as Node | null
  if (autoRefreshDropdownRef.value && target && !autoRefreshDropdownRef.value.contains(target)) {
    showAutoRefreshDropdown.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleAutoRefreshClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleAutoRefreshClickOutside)
})
</script>
