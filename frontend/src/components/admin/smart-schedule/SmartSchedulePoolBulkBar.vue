<template>
  <section data-testid="smart-schedule-bulk-region">
    <div
      class="flex items-center justify-between rounded-lg bg-primary-50 px-3 py-2 dark:bg-primary-900/20"
      data-testid="smart-schedule-pool-bulk-bar"
    >
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm font-medium text-primary-900 dark:text-primary-100">
          {{
            selectedIds.length > 0
              ? t('admin.accounts.bulkActions.selected', { count: selectedIds.length })
              : t('admin.users.smartSchedule.bulkTitle')
          }}
        </span>
        <template v-if="filteredCount > 0">
          <button
            type="button"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
            data-testid="smart-schedule-select-page"
            @click="$emit('select-page')"
          >
            {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
          </button>
          <span class="text-gray-300 dark:text-primary-800">/</span>
          <button
            type="button"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
            data-testid="smart-schedule-select-matching"
            @click="$emit('select-matching')"
          >
            {{ t('admin.accounts.bulkActions.selectFiltered', { count: filteredCount }) }}
          </button>
          <span class="text-gray-300 dark:text-primary-800">/</span>
          <button
            type="button"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
            data-testid="smart-schedule-select-oauth"
            @click="$emit('select-oauth')"
          >
            {{ t('admin.users.smartSchedule.selectAllOauth') }}
          </button>
          <span class="text-gray-300 dark:text-primary-800">/</span>
          <button
            type="button"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
            data-testid="smart-schedule-select-apikey"
            @click="$emit('select-apikey')"
          >
            {{ t('admin.users.smartSchedule.selectAllApikey') }}
          </button>
          <template v-if="selectedIds.length > 0">
            <span class="text-gray-300 dark:text-primary-800">/</span>
            <button
              type="button"
              class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
              data-testid="smart-schedule-clear-selection"
              @click="$emit('clear')"
            >
              {{ t('admin.accounts.bulkActions.clear') }}
            </button>
          </template>
        </template>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <input
          :value="bulkCap ?? ''"
          type="number"
          min="1"
          class="input w-20"
          data-testid="smart-schedule-bulk-cap-input"
          :placeholder="t('admin.users.smartSchedule.applyCap')"
          @input="$emit('update:bulkCap', Number(($event.target as HTMLInputElement).value) || null)"
        />
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="!bulkCap || bulkCap < 1"
          data-testid="smart-schedule-apply-cap"
          @click="$emit('apply-cap-all')"
        >
          {{ t('admin.users.smartSchedule.applyCapButton') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="admissionIds.length === 0"
          data-testid="smart-schedule-batch-paused"
          @click="$emit('apply-admission', 'paused')"
        >
          {{ t('admin.users.smartSchedule.batchPaused') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="admissionIds.length === 0"
          data-testid="smart-schedule-batch-selectable"
          @click="$emit('apply-admission', 'selectable')"
        >
          {{ t('admin.users.smartSchedule.batchSelectable') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="admissionIds.length === 0"
          data-testid="smart-schedule-batch-probing"
          @click="$emit('apply-admission', 'probing')"
        >
          {{ t('admin.users.smartSchedule.batchProbing') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="selectedIds.length === 0 || !bulkCap || bulkCap < 1"
          data-testid="smart-schedule-batch-apply-cap"
          @click="$emit('apply-cap')"
        >
          {{ t('admin.users.smartSchedule.batchApplyCap') }}
        </button>
        <button
          type="button"
          class="btn btn-danger btn-sm"
          :disabled="selectedIds.length === 0"
          data-testid="smart-schedule-batch-remove"
          @click="$emit('remove')"
        >
          {{ t('admin.users.smartSchedule.batchRemove') }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

withDefaults(defineProps<{
  selectedIds: number[]
  admissionIds?: number[]
  filteredCount: number
  bulkCap: number | null
}>(), {
  admissionIds: () => []
})

defineEmits<{
  'select-page': []
  'select-matching': []
  'select-oauth': []
  'select-apikey': []
  clear: []
  'apply-admission': [state: 'paused' | 'selectable' | 'probing']
  'apply-cap': []
  'apply-cap-all': []
  remove: []
  'update:bulkCap': [value: number | null]
}>()

const { t } = useI18n()
</script>
