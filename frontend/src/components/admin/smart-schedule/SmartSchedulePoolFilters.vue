<template>
  <section
    class="rounded-lg border border-gray-200 bg-white px-1.5 py-1 dark:border-dark-600 dark:bg-dark-800"
    data-testid="smart-schedule-filter-region"
    :title="t('admin.users.smartSchedule.filterRegionHint')"
  >
    <div class="flex flex-wrap items-center gap-1.5" data-testid="smart-schedule-pool-filters">
      <span class="shrink-0 text-xs font-medium uppercase tracking-wide text-gray-400">
        {{ t('admin.users.smartSchedule.filterRegionTitle') }}
      </span>
      <SearchInput
        :model-value="filters.search"
        :placeholder="t('admin.users.smartSchedule.filterSearch')"
        class="w-full sm:w-52"
        data-testid="smart-schedule-pool-search"
        @update:model-value="patch('search', $event)"
      />
      <Select
        :model-value="filters.type"
        class="w-36"
        :options="typeOptions"
        data-testid="smart-schedule-filter-type"
        @update:model-value="patch('type', String($event ?? ''))"
      />
      <Select
        :model-value="filters.schedulable"
        class="w-36"
        :options="schedulableOptions"
        data-testid="smart-schedule-filter-schedulable"
        @update:model-value="patch('schedulable', String($event ?? '') as SmartSchedulePoolFilters['schedulable'])"
      />
      <Select
        :model-value="filters.admission"
        class="w-44"
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
import {
  POOL_ADMISSION_FILTER_STATES,
  type SmartSchedulePoolFilters
} from '@/composables/smartSchedulePoolAdmission'

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

const admissionLabelKey: Record<(typeof POOL_ADMISSION_FILTER_STATES)[number], string> = {
  selectable: 'admin.users.smartSchedule.admissionSelectable',
  probing: 'admin.users.smartSchedule.admissionProbing',
  resumed: 'admin.users.smartSchedule.admissionResumed',
  pinned: 'admin.users.smartSchedule.admissionPinned',
  will_cool: 'admin.users.smartSchedule.admissionWillCool',
  cooling: 'admin.users.smartSchedule.admissionCooling',
  paused: 'admin.users.smartSchedule.admissionPaused',
  pair_full: 'admin.users.smartSchedule.admissionPairFull',
  stopped: 'admin.users.smartSchedule.admissionStopped',
  unsaved_preview: 'admin.users.smartSchedule.admissionUnsavedPreview'
}

const admissionOptions = computed(() => [
  { value: '', label: t('admin.users.smartSchedule.filterAdmissionAll') },
  ...POOL_ADMISSION_FILTER_STATES.map((value) => ({
    value,
    label: t(admissionLabelKey[value])
  }))
])

function patch<K extends keyof SmartSchedulePoolFilters>(key: K, value: SmartSchedulePoolFilters[K]) {
  emit('update:filters', { ...props.filters, [key]: value })
}
</script>
