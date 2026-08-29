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
        <span class="text-gray-300 dark:text-primary-800">/</span>
        <div class="relative" ref="upstreamRateSelectRef">
          <button
            type="button"
            data-testid="select-by-upstream-rate"
            :disabled="selectingAllFiltered"
            class="text-xs font-medium text-primary-700 hover:text-primary-800 disabled:cursor-not-allowed disabled:opacity-60 dark:text-primary-300 dark:hover:text-primary-200"
            @click="showUpstreamRateSelect = !showUpstreamRateSelect"
          >
            {{ t('admin.accounts.bulkActions.selectByUpstreamRate') }}
          </button>
          <div
            v-if="showUpstreamRateSelect"
            class="absolute left-0 z-50 mt-2 w-64 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-gray-700 dark:bg-gray-800"
            data-testid="select-by-upstream-rate-panel"
          >
            <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.bulkActions.selectByUpstreamRateHint') }}
            </p>
            <div class="flex items-center gap-2">
              <select
                v-model="upstreamRateComparison"
                data-testid="select-by-upstream-rate-comparison"
                class="input h-8 w-24 py-0 text-xs"
              >
                <option value="lt">{{ t('admin.accounts.bulkActions.selectByUpstreamRateBelow') }}</option>
                <option value="gt">{{ t('admin.accounts.bulkActions.selectByUpstreamRateAbove') }}</option>
              </select>
              <input
                v-model.number="upstreamRateThreshold"
                data-testid="select-by-upstream-rate-threshold"
                type="number"
                min="0"
                step="0.01"
                class="input h-8 min-w-0 flex-1 py-0 text-xs"
              />
            </div>
            <button
              type="button"
              data-testid="select-by-upstream-rate-apply"
              class="btn btn-primary btn-sm mt-2 w-full"
              :disabled="selectingAllFiltered || !isFiniteUpstreamRateThreshold"
              @click="applyUpstreamRateSelect"
            >
              {{ t('admin.accounts.bulkActions.selectByUpstreamRateApply') }}
            </button>
          </div>
        </div>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UpstreamRateComparison } from '@/utils/accountUpstreamRate'

withDefaults(defineProps<{
  selectedIds: number[]
  total?: number
  selectingAllFiltered?: boolean
}>(), {
  total: 0,
  selectingAllFiltered: false
})

const emit = defineEmits<{
  delete: []
  'edit-selected': []
  'edit-filtered': []
  clear: []
  'select-page': []
  'select-filtered': []
  'toggle-schedulable': [enabled: boolean]
  'reset-status': []
  'refresh-token': []
  'auto-assign-proxy': []
  'unbind-subscription-by-rate': []
  'select-by-upstream-rate': [payload: { comparison: UpstreamRateComparison; threshold: number }]
}>()

const { t } = useI18n()
const showUpstreamRateSelect = ref(false)
const upstreamRateComparison = ref<UpstreamRateComparison>('lt')
const upstreamRateThreshold = ref(1)
const upstreamRateSelectRef = ref<HTMLElement | null>(null)

const isFiniteUpstreamRateThreshold = computed(() => Number.isFinite(Number(upstreamRateThreshold.value)))

function applyUpstreamRateSelect() {
  if (!isFiniteUpstreamRateThreshold.value) return
  emit('select-by-upstream-rate', {
    comparison: upstreamRateComparison.value,
    threshold: Number(upstreamRateThreshold.value)
  })
  showUpstreamRateSelect.value = false
}

function handleDocumentClick(event: MouseEvent) {
  const root = upstreamRateSelectRef.value
  if (!root || root.contains(event.target as Node)) return
  showUpstreamRateSelect.value = false
}

onMounted(() => {
  document.addEventListener('mousedown', handleDocumentClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', handleDocumentClick)
})
</script>
