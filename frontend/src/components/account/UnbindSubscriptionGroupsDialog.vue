<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.unbindSubscriptionByRate.title')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('admin.accounts.unbindSubscriptionByRate.description') }}
      </p>

      <div class="grid items-start gap-4 sm:grid-cols-2">
        <label class="block">
          <span class="input-label">{{ t('admin.accounts.unbindSubscriptionByRate.minRate') }}</span>
          <input
            v-model.number="minRate"
            data-testid="unbind-subscription-threshold"
            type="number"
            min="0"
            step="0.01"
            class="input h-10 py-0 leading-5"
          />
        </label>
        <div class="unbind-platform-select block">
          <span class="input-label">{{ t('admin.accounts.unbindSubscriptionByRate.platform') }}</span>
          <Select
            v-model="platform"
            :options="platformOptions"
            data-testid="unbind-subscription-platform"
          />
        </div>
      </div>
      <p class="input-hint">
        {{ t('admin.accounts.unbindSubscriptionByRate.minRateHint') }}
      </p>

      <label class="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200">
        <input
          v-model="allowEmptyGroups"
          data-testid="unbind-subscription-allow-empty"
          type="checkbox"
          class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <span>
          <span class="font-medium">{{ t('admin.accounts.unbindSubscriptionByRate.allowEmpty') }}</span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.unbindSubscriptionByRate.allowEmptyHint') }}
          </span>
        </span>
      </label>

      <p v-if="previewStale && result" class="text-sm text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.unbindSubscriptionByRate.stalePreview') }}
      </p>

      <div v-if="result" class="flex flex-wrap gap-2 text-xs">
        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-700">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.matched', { count: result.matched }) }}
        </span>
        <span class="rounded-full bg-primary-50 px-2 py-1 text-primary-800 dark:bg-primary-900/30 dark:text-primary-200">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.wouldApply', { count: result.would_apply }) }}
        </span>
        <span class="rounded-full bg-gray-100 px-2 py-1 dark:bg-dark-700">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.skippedNoSubscription', { count: result.skipped_no_subscription }) }}
        </span>
        <span class="rounded-full bg-amber-50 px-2 py-1 text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.skippedEmpty', { count: result.skipped_would_be_empty }) }}
        </span>
        <span v-if="!lastWasPreview" class="rounded-full bg-emerald-50 px-2 py-1 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.applied', { count: result.applied }) }}
        </span>
        <span v-if="!lastWasPreview" class="rounded-full bg-red-50 px-2 py-1 text-red-800 dark:bg-red-900/30 dark:text-red-200">
          {{ t('admin.accounts.unbindSubscriptionByRate.counts.failed', { count: result.failed }) }}
        </span>
      </div>

      <div v-if="result" class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
        <table data-testid="unbind-subscription-table" class="min-w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.name') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.id') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.rate') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.remove') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.keep') }}</th>
              <th class="px-3 py-2">{{ t('admin.accounts.unbindSubscriptionByRate.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in result.accounts"
              :key="row.id"
              class="border-t border-gray-100 dark:border-dark-700"
            >
              <td class="px-3 py-2 font-medium text-gray-900 dark:text-gray-100">{{ row.name }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ row.id }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ row.rate }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ formatGroups(row.remove_groups) }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">{{ formatGroups(row.keep_groups) }}</td>
              <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                {{ t(`admin.accounts.unbindSubscriptionByRate.action.${row.action}`) }}
                <span v-if="row.error" class="mt-1 block text-xs text-red-600 dark:text-red-300">{{ row.error }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="handleClose">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        data-testid="unbind-subscription-preview"
        class="btn btn-secondary"
        :disabled="previewing || applying || !isFiniteThreshold"
        @click="runPreview"
      >
        {{ previewing ? t('admin.accounts.unbindSubscriptionByRate.previewing') : t('admin.accounts.unbindSubscriptionByRate.preview') }}
      </button>
      <button
        type="button"
        data-testid="unbind-subscription-apply"
        class="btn btn-danger"
        :disabled="!canApply"
        @click="runApply"
      >
        {{ applying
          ? t('admin.accounts.unbindSubscriptionByRate.applying')
          : t('admin.accounts.unbindSubscriptionByRate.apply', { count: result?.would_apply ?? 0 }) }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { UnbindSubscriptionGroupRef, UnbindSubscriptionGroupsByRateResult } from '@/api/admin/accounts'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
  applied: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const minRate = ref(1)
const platform = ref('')
const allowEmptyGroups = ref(false)
const previewing = ref(false)
const applying = ref(false)
const previewStale = ref(false)
const lastWasPreview = ref(true)
const result = ref<UnbindSubscriptionGroupsByRateResult | null>(null)

const platformOptions = computed(() => [
  { value: '', label: t('admin.accounts.unbindSubscriptionByRate.platformAll') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' }
])

const isFiniteThreshold = computed(() => Number.isFinite(Number(minRate.value)))
const canApply = computed(() => {
  return Boolean(
    result.value &&
    !previewStale.value &&
    !previewing.value &&
    !applying.value &&
    result.value.would_apply > 0 &&
    lastWasPreview.value
  )
})

watch(
  () => [minRate.value, platform.value, allowEmptyGroups.value],
  () => {
    if (result.value) previewStale.value = true
  }
)

watch(
  () => props.show,
  (open) => {
    if (!open) return
    minRate.value = 1
    platform.value = ''
    allowEmptyGroups.value = false
    result.value = null
    previewStale.value = false
    lastWasPreview.value = true
  }
)

function formatGroups(groups: UnbindSubscriptionGroupRef[] | undefined): string {
  if (!groups || groups.length === 0) return t('common.none')
  return groups.map((group) => group.name || String(group.id)).join(', ')
}

function currentPayload(dryRun: boolean) {
  return {
    min_rate_multiplier: Number(minRate.value),
    platform: platform.value,
    dry_run: dryRun,
    allow_empty_groups: allowEmptyGroups.value
  }
}

async function runPreview() {
  if (!isFiniteThreshold.value) return
  previewing.value = true
  try {
    result.value = await adminAPI.accounts.unbindSubscriptionGroupsByRate(currentPayload(true))
    previewStale.value = false
    lastWasPreview.value = true
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.unbindSubscriptionByRate.previewFailed'))
  } finally {
    previewing.value = false
  }
}

async function runApply() {
  if (!canApply.value) return
  applying.value = true
  try {
    result.value = await adminAPI.accounts.unbindSubscriptionGroupsByRate(currentPayload(false))
    previewStale.value = false
    lastWasPreview.value = false
    appStore.showSuccess(t('admin.accounts.unbindSubscriptionByRate.applySuccess', {
      applied: result.value.applied,
      failed: result.value.failed
    }))
    emit('applied')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.unbindSubscriptionByRate.applyFailed'))
  } finally {
    applying.value = false
  }
}

function handleClose() {
  emit('close')
}
</script>

<style scoped>
.unbind-platform-select :deep(.select-trigger) {
  @apply h-10 py-0 leading-5;
}

.unbind-platform-select :deep(.select-value) {
  @apply leading-5;
}
</style>
