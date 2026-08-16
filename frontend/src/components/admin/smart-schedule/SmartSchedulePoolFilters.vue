<template>
  <section
    class="space-y-3 rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800"
    data-testid="smart-schedule-filter-region"
  >
    <div>
      <p class="text-xs font-medium uppercase tracking-wide text-gray-400">
        {{ t('admin.users.smartSchedule.filterRegionTitle') }}
      </p>
      <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.users.smartSchedule.filterRegionHint') }}
      </p>
    </div>
    <div class="flex flex-wrap items-center gap-3" data-testid="smart-schedule-pool-filters">
      <SearchInput
        :model-value="filters.search"
        :placeholder="t('admin.users.smartSchedule.filterSearch')"
        class="w-full sm:w-64"
        data-testid="smart-schedule-pool-search"
        @update:model-value="patch('search', $event)"
      />
      <Select
        :model-value="filters.type"
        class="w-40"
        :options="typeOptions"
        data-testid="smart-schedule-filter-type"
        @update:model-value="patch('type', String($event ?? ''))"
      />
      <Select
        :model-value="filters.schedulable"
        class="w-40"
        :options="schedulableOptions"
        data-testid="smart-schedule-filter-schedulable"
        @update:model-value="patch('schedulable', String($event ?? '') as SmartSchedulePoolFilters['schedulable'])"
      />
      <Select
        :model-value="filters.admission"
        class="w-40"
        :options="admissionOptions"
        data-testid="smart-schedule-filter-admission"
        @update:model-value="patch('admission', String($event ?? '') as SmartSchedulePoolFilters['admission'])"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import type { SmartSchedulePoolFilters } from '@/composables/smartSchedulePoolAdmission'

const props = defineProps<{
  filters: SmartSchedulePoolFilters
}>()

const emit = defineEmits<{
  'update:filters': [value: SmartSchedulePoolFilters]
}>()

const { t } = useI18n()

const typeOptions = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'apikey', label: t('admin.accounts.apiKey') },
  { value: 'setup-token', label: t('admin.accounts.setupToken') }
])

const schedulableOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterSchedulableAll') },
  { value: 'on', label: t('admin.users.smartSchedule.filterSchedulableOn') },
  { value: 'off', label: t('admin.users.smartSchedule.filterSchedulableOff') }
])

const admissionOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterAdmissionAll') },
  { value: 'selectable', label: t('admin.users.smartSchedule.admissionSelectable') },
  { value: 'cooling', label: t('admin.users.smartSchedule.admissionCooling') },
  { value: 'pair_full', label: t('admin.users.smartSchedule.admissionPairFull') },
  { value: 'stopped', label: t('admin.users.smartSchedule.admissionStopped') },
  { value: 'quality_blocked', label: t('admin.users.smartSchedule.admissionQualityBlocked') }
])

function patch<K extends keyof SmartSchedulePoolFilters>(key: K, value: SmartSchedulePoolFilters[K]) {
  emit('update:filters', { ...props.filters, [key]: value })
}
</script>
