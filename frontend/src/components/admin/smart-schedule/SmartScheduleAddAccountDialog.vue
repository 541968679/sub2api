<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.smartSchedule.filteredAddTitle')"
    width="extra-wide"
    :close-on-click-outside="true"
    data-testid="smart-schedule-add-dialog"
    @close="$emit('close')"
  >
    <div class="space-y-4" data-testid="smart-schedule-add-dialog">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.users.smartSchedule.filteredAddHint', { platform: platformLabel }) }}
      </p>

      <AccountTableFilters
        :search-query="filters.search"
        :filters="filters"
        :groups="groups"
        hide-platform
        :show-sort="false"
        :search-placeholder="t('admin.users.smartSchedule.addAccountPlaceholder')"
        @update:search-query="patchSearch"
        @update:filters="onListFilters"
      >
        <template #extra>
          <Select
            :model-value="filters.schedulable"
            class="w-40"
            :options="schedulableOptions"
            data-testid="smart-schedule-add-filter-schedulable"
            @update:model-value="patch('schedulable', String($event ?? '') as SmartScheduleAddCandidateFilters['schedulable'])"
          />
          <Select
            :model-value="filters.scheduling"
            class="w-44"
            :options="schedulingOptions"
            data-testid="smart-schedule-add-filter-scheduling"
            @update:model-value="patch('scheduling', String($event ?? '') as SmartScheduleAddCandidateFilters['scheduling'])"
          />
          <Select
            :model-value="filters.proxy"
            class="w-44"
            :options="proxyOptions"
            data-testid="smart-schedule-add-filter-proxy"
            @update:model-value="patch('proxy', String($event ?? ''))"
          />
        </template>
      </AccountTableFilters>

      <div class="flex flex-wrap items-center gap-2">
        <span class="text-xs text-gray-500">{{ t('admin.users.smartSchedule.addScheduling') }}</span>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="smart-schedule-add-dialog-preset-api"
          @click="applyPreset('apikey')"
        >
          {{ t('admin.users.smartSchedule.addSchedulingApi', { count: schedulingApiCount }) }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="smart-schedule-add-dialog-preset-oauth"
          @click="applyPreset('oauth')"
        >
          {{ t('admin.users.smartSchedule.addSchedulingOauth', { count: schedulingOauthCount }) }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          data-testid="smart-schedule-add-dialog-preset-all"
          @click="applyPreset('all')"
        >
          {{ t('admin.users.smartSchedule.addSchedulingAll', { count: schedulingAllCount }) }}
        </button>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-sm font-medium text-gray-800 dark:text-gray-100">
          {{ t('admin.users.smartSchedule.filteredAddPreview', { count: matchingAccounts.length }) }}
        </p>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <button
            type="button"
            class="font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300"
            data-testid="smart-schedule-add-dialog-select-all"
            :disabled="matchingAccounts.length === 0"
            @click="selectAllMatching"
          >
            {{ t('admin.accounts.bulkActions.selectFiltered', { count: matchingAccounts.length }) }}
          </button>
          <button
            v-if="selectedIds.length > 0"
            type="button"
            class="font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300"
            data-testid="smart-schedule-add-dialog-clear"
            @click="selectedIds = []"
          >
            {{ t('admin.accounts.bulkActions.clear') }}
          </button>
        </div>
      </div>

      <div
        class="max-h-80 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600"
        data-testid="smart-schedule-add-dialog-preview"
      >
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-600">
          <thead class="sticky top-0 bg-gray-50 dark:bg-dark-700">
            <tr class="text-left text-xs text-gray-500">
              <th class="w-10 px-3 py-2">
                <input
                  type="checkbox"
                  class="rounded border-gray-300 text-primary-600"
                  :checked="allMatchingSelected"
                  :disabled="matchingAccounts.length === 0"
                  data-testid="smart-schedule-add-dialog-select-header"
                  @change="toggleSelectAll"
                />
              </th>
              <th class="px-3 py-2">{{ t('admin.accounts.columns.name') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.columns.platformType') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.columns.status') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.columns.schedulable') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.columns.groups') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="matchingAccounts.length === 0">
              <td colspan="6" class="px-3 py-8 text-center text-gray-500">
                {{ t('admin.users.smartSchedule.filteredAddEmpty') }}
              </td>
            </tr>
            <tr
              v-for="account in matchingAccounts"
              :key="account.id"
              class="border-t border-gray-100 dark:border-dark-700"
              :data-testid="`smart-schedule-add-dialog-row-${account.id}`"
            >
              <td class="px-3 py-2">
                <input
                  type="checkbox"
                  class="rounded border-gray-300 text-primary-600"
                  :checked="selectedIds.includes(account.id)"
                  :data-testid="`smart-schedule-add-dialog-check-${account.id}`"
                  @change="toggleId(account.id)"
                />
              </td>
              <td class="px-3 py-2">
                <div class="font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
                <div class="text-xs text-gray-500">#{{ account.id }}</div>
              </td>
              <td class="px-3 py-2">
                <PlatformTypeBadge :platform="account.platform" :type="account.type" />
              </td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ account.status }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                {{
                  account.schedulable
                    ? t('admin.users.smartSchedule.filterSchedulableOn')
                    : t('admin.users.smartSchedule.filterSchedulableOff')
                }}
              </td>
              <td class="px-3 py-2">
                <AccountGroupsCell :groups="account.groups" :max-display="3" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <template #footer>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <button type="button" class="btn btn-secondary" data-testid="smart-schedule-add-dialog-cancel" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="matchingAccounts.length === 0"
          data-testid="smart-schedule-add-dialog-all"
          @click="addAllMatching"
        >
          {{ t('admin.users.smartSchedule.addAllMatching', { count: matchingAccounts.length }) }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="selectedIds.length === 0"
          data-testid="smart-schedule-add-dialog-selected"
          @click="addSelected"
        >
          {{ t('admin.users.smartSchedule.addSelected', { count: selectedIds.length }) }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import AccountTableFilters from '@/components/admin/account/AccountTableFilters.vue'
import {
  emptySmartScheduleAddFilters,
  filterAddCandidates,
  type SmartScheduleAddCandidateFilters
} from '@/composables/smartScheduleAddCandidates'
import { isCurrentlySchedulingAccount } from '@/composables/smartSchedulePoolAdmission'
import type { Account, AdminGroup, Proxy } from '@/types'

const props = defineProps<{
  show: boolean
  platform: string
  platformLabel: string
  accounts: Account[]
  groups: AdminGroup[]
  proxies: Proxy[]
}>()

const emit = defineEmits<{
  close: []
  add: [accountIds: number[]]
}>()

const { t } = useI18n()
const filters = ref<SmartScheduleAddCandidateFilters>(emptySmartScheduleAddFilters(props.platform))
const selectedIds = ref<number[]>([])

const matchingAccounts = computed(() => filterAddCandidates(props.accounts, filters.value))
const matchingIdSet = computed(() => new Set(matchingAccounts.value.map((account) => account.id)))
const allMatchingSelected = computed(
  () => matchingAccounts.value.length > 0 && matchingAccounts.value.every((account) => selectedIds.value.includes(account.id))
)
const schedulingApiCount = computed(
  () => props.accounts.filter((account) => account.type === 'apikey' && isCurrentlySchedulingAccount(account)).length
)
const schedulingOauthCount = computed(
  () => props.accounts.filter((account) => account.type === 'oauth' && isCurrentlySchedulingAccount(account)).length
)
const schedulingAllCount = computed(
  () => props.accounts.filter((account) => isCurrentlySchedulingAccount(account)).length
)

const schedulableOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterSchedulableAll') },
  { value: 'on', label: t('admin.users.smartSchedule.filterSchedulableOn') },
  { value: 'off', label: t('admin.users.smartSchedule.filterSchedulableOff') }
])

const schedulingOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterSchedulingAll') },
  { value: 'on', label: t('admin.users.smartSchedule.filterSchedulingOn') },
  { value: 'off', label: t('admin.users.smartSchedule.filterSchedulingOff') }
])

const proxyOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterProxyAll') },
  { value: 'none', label: t('admin.users.smartSchedule.filterProxyNone') },
  ...props.proxies.map((proxy) => ({ value: String(proxy.id), label: proxy.name }))
])

watch(
  () => [props.show, props.platform] as const,
  ([show, platform]) => {
    if (!show) return
    filters.value = emptySmartScheduleAddFilters(platform)
    selectedIds.value = []
  }
)

function patchSearch(value: string) {
  filters.value = { ...filters.value, search: value }
}

function onListFilters(next: Record<string, unknown>) {
  filters.value = {
    ...filters.value,
    ...next,
    platform: props.platform,
    search: filters.value.search,
    schedulable: filters.value.schedulable,
    scheduling: filters.value.scheduling,
    proxy: filters.value.proxy
  }
}

function patch<K extends keyof SmartScheduleAddCandidateFilters>(key: K, value: SmartScheduleAddCandidateFilters[K]) {
  filters.value = { ...filters.value, [key]: value }
}

function applyPreset(scope: 'apikey' | 'oauth' | 'all') {
  filters.value = {
    ...emptySmartScheduleAddFilters(props.platform),
    type: scope === 'all' ? '' : scope,
    scheduling: 'on',
    status: 'active'
  }
  selectedIds.value = []
}

function toggleId(accountId: number) {
  if (selectedIds.value.includes(accountId)) {
    selectedIds.value = selectedIds.value.filter((id) => id !== accountId)
    return
  }
  selectedIds.value = [...selectedIds.value, accountId]
}

function selectAllMatching() {
  selectedIds.value = matchingAccounts.value.map((account) => account.id)
}

function toggleSelectAll(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedIds.value = checked ? matchingAccounts.value.map((account) => account.id) : []
}

function addSelected() {
  const ids = selectedIds.value.filter((id) => matchingIdSet.value.has(id))
  if (ids.length === 0) return
  emit('add', ids)
}

function addAllMatching() {
  const ids = matchingAccounts.value.map((account) => account.id)
  if (ids.length === 0) return
  emit('add', ids)
}
</script>
