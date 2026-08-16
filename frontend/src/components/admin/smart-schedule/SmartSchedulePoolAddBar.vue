<template>
  <section
    class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-600 dark:bg-dark-800/60"
    data-testid="smart-schedule-add-region"
  >
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="min-w-0">
        <p class="text-xs font-medium uppercase tracking-wide text-gray-400">
          {{ t('admin.users.smartSchedule.addRegionTitle') }}
        </p>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.users.smartSchedule.addRegionHint') }}
        </p>
      </div>
      <button
        type="button"
        class="btn btn-primary btn-sm"
        data-testid="smart-schedule-filtered-add"
        @click="$emit('open-filtered-add')"
      >
        {{ t('admin.users.smartSchedule.filteredAdd') }}
      </button>
    </div>

    <div class="flex flex-wrap items-end gap-2">
      <label class="relative min-w-[16rem] flex-1 space-y-1">
        <span class="block text-xs text-gray-500">{{ t('admin.users.smartSchedule.addAccount') }}</span>
        <input
          :value="searchQuery"
          type="text"
          class="input w-full"
          data-testid="smart-schedule-add-select"
          :placeholder="t('admin.users.smartSchedule.addAccountPlaceholder')"
          autocomplete="off"
          @focus="$emit('update:searchOpen', true)"
          @input="onSearchInput"
          @blur="$emit('search-blur')"
        />
        <div
          v-if="searchOpen"
          class="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
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
      </label>
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

    <div class="flex flex-wrap items-center gap-2">
      <span class="text-xs text-gray-500">{{ t('admin.users.smartSchedule.addScheduling') }}</span>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="apiCount === 0"
        data-testid="smart-schedule-add-api"
        @click="$emit('add-scheduling', 'apikey')"
      >
        {{ t('admin.users.smartSchedule.addSchedulingApi', { count: apiCount }) }}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="oauthCount === 0"
        data-testid="smart-schedule-add-oauth"
        @click="$emit('add-scheduling', 'oauth')"
      >
        {{ t('admin.users.smartSchedule.addSchedulingOauth', { count: oauthCount }) }}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="allCount === 0"
        data-testid="smart-schedule-add-all"
        @click="$emit('add-scheduling', 'all')"
      >
        {{ t('admin.users.smartSchedule.addSchedulingAll', { count: allCount }) }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { Account } from '@/types'
import type { SmartScheduleAddScope } from '@/composables/useUserSmartScheduleEditor'

defineProps<{
  searchQuery: string
  searchOpen: boolean
  filteredAccounts: Account[]
  apiCount: number
  oauthCount: number
  allCount: number
  poolEmpty: boolean
  autoSorting: boolean
  autoSortDone: number
  autoSortTotal: number
}>()

const emit = defineEmits<{
  'update:searchQuery': [value: string]
  'update:searchOpen': [value: boolean]
  'search-blur': []
  choose: [accountId: number]
  'open-filtered-add': []
  'add-scheduling': [scope: SmartScheduleAddScope]
  'auto-sort': []
}>()

const { t } = useI18n()

function onSearchInput(event: Event) {
  emit('update:searchQuery', (event.target as HTMLInputElement).value)
  emit('update:searchOpen', true)
}
</script>
