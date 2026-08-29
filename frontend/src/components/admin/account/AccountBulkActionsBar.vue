<template>
  <div class="mb-4 flex items-center justify-between rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20">
    <div class="flex flex-wrap items-center gap-2">
      <span v-if="selectedIds.length > 0" class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkActions.selected', { count: selectedIds.length }) }}
      </span>
      <span v-else class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkEdit.title') }}
      </span>
      <template v-if="total > 0">
        <button
          @click="$emit('select-page')"
          class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
        >
          {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
        </button>
        <span class="text-gray-300 dark:text-primary-800">/</span>
        <button
          :disabled="selectingAllFiltered"
          @click="$emit('select-filtered')"
          class="text-xs font-medium text-primary-700 hover:text-primary-800 disabled:cursor-not-allowed disabled:opacity-60 dark:text-primary-300 dark:hover:text-primary-200"
        >
          {{
            selectingAllFiltered
              ? t('admin.accounts.bulkActions.selectingFiltered')
              : t('admin.accounts.bulkActions.selectFiltered', { count: total })
          }}
        </button>
        <template v-if="selectedIds.length > 0">
          <span class="text-gray-300 dark:text-primary-800">/</span>
          <button
            @click="$emit('clear')"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
          >
            {{ t('admin.accounts.bulkActions.clear') }}
          </button>
        </template>
      </template>
    </div>
    <div class="flex flex-wrap gap-2">
      <template v-if="selectedIds.length > 0">
        <button @click="$emit('delete')" class="btn btn-danger btn-sm">{{ t('admin.accounts.bulkActions.delete') }}</button>
        <button @click="$emit('reset-status')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.resetStatus') }}</button>
        <button @click="$emit('refresh-token')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.refreshToken') }}</button>
        <button @click="$emit('toggle-schedulable', true)" class="btn btn-success btn-sm">{{ t('admin.accounts.bulkActions.enableScheduling') }}</button>
        <button @click="$emit('toggle-schedulable', false)" class="btn btn-warning btn-sm">{{ t('admin.accounts.bulkActions.disableScheduling') }}</button>
        <button
          data-testid="bulk-edit-selected"
          @click="$emit('edit-selected')"
          class="btn btn-primary btn-sm"
        >
          {{ t('admin.accounts.bulkActions.editSelected') }}
        </button>
        <button @click="$emit('auto-assign-proxy')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.autoAssignProxy') }}</button>
      </template>
      <!-- Same BulkEditAccountModal as edit-selected; scope is current filters (cross-page). -->
      <button
        data-testid="bulk-edit-filtered"
        @click="$emit('edit-filtered')"
        class="btn btn-sm"
        :class="selectedIds.length > 0 ? 'btn-secondary' : 'btn-primary'"
      >
        {{ t('admin.accounts.bulkActions.editFiltered') }}
      </button>
      <button
        data-testid="unbind-subscription-by-rate"
        @click="$emit('unbind-subscription-by-rate')"
        class="btn btn-secondary btn-sm"
      >
        {{ t('admin.accounts.unbindSubscriptionByRate.button') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(defineProps<{
  selectedIds: number[]
  total?: number
  selectingAllFiltered?: boolean
}>(), {
  total: 0,
  selectingAllFiltered: false
})

defineEmits([
  'delete',
  'edit-selected',
  'edit-filtered',
  'clear',
  'select-page',
  'select-filtered',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'auto-assign-proxy',
  'unbind-subscription-by-rate'
])

const { t } = useI18n()
</script>
