<template>
  <div
    class="flex h-full w-full min-h-0 flex-col justify-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 shadow-sm dark:border-amber-700/40 dark:bg-amber-900/20"
    data-test="public-schedule-quality-global"
  >
    <div class="flex items-center gap-2">
      <div class="min-w-0 flex-1">
        <h3
          class="truncate text-sm font-semibold leading-5 text-amber-900 dark:text-amber-100"
          :title="t('admin.accounts.publicQuality.hint')"
        >
          {{ t('admin.accounts.publicQuality.title') }}
        </h3>
      </div>
      <Toggle
        v-model="form.enabled"
        data-test="public-quality-enabled"
        :title="t('admin.accounts.publicQuality.enabledHint')"
      />
      <button
        type="button"
        class="btn btn-primary shrink-0 px-2.5 py-1 text-xs"
        data-test="public-quality-save"
        :disabled="saving || loading"
        @click="save"
      >
        {{ saving ? t('common.saving') : t('admin.accounts.publicQuality.saveShort') }}
      </button>
    </div>

    <div v-if="loading" class="text-xs leading-4 text-amber-800/70 dark:text-amber-200/70">
      {{ t('common.loading') }}
    </div>

    <div v-else class="grid grid-cols-3 gap-x-2 gap-y-1">
      <label class="min-w-0">
        <span class="mb-0.5 block truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
          {{ t('admin.accounts.publicQuality.ttftN') }}
        </span>
        <input
          v-model="form.ttft_window_n"
          type="number"
          min="1"
          max="100"
          class="input h-7 w-full rounded-lg px-1.5 py-0 text-xs"
          data-test="public-quality-ttft-n"
        />
      </label>
      <label class="min-w-0">
        <span class="mb-0.5 block truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
          {{ t('admin.accounts.publicQuality.successN') }}
        </span>
        <input
          v-model="form.success_window_n"
          type="number"
          min="1"
          max="100"
          class="input h-7 w-full rounded-lg px-1.5 py-0 text-xs"
          data-test="public-quality-success-n"
        />
      </label>
      <label class="min-w-0">
        <span
          class="mb-0.5 block truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100"
          :title="t('admin.accounts.publicQuality.maxP50')"
        >
          {{ t('admin.accounts.publicQuality.maxP50Short') }}
        </span>
        <input
          v-model="form.quality_max_p50_ttft_ms"
          type="number"
          min="1"
          class="input h-7 w-full rounded-lg px-1.5 py-0 text-xs"
          data-test="public-quality-p50"
        />
      </label>
      <label class="min-w-0">
        <span
          class="mb-0.5 block truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100"
          :title="t('admin.accounts.publicQuality.minSuccessRate')"
        >
          {{ t('admin.accounts.publicQuality.minSuccessRateShort') }}
        </span>
        <input
          v-model="form.min_success_rate_percent"
          type="number"
          min="0.1"
          max="100"
          step="0.1"
          class="input h-7 w-full rounded-lg px-1.5 py-0 text-xs"
          data-test="public-quality-success-rate"
        />
      </label>
      <label class="min-w-0">
        <span
          class="mb-0.5 block truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100"
          :title="t('admin.accounts.publicQuality.cooldownMinutes')"
        >
          {{ t('admin.accounts.publicQuality.cooldownShort') }}
        </span>
        <input
          v-model="form.cooldown_minutes"
          type="number"
          min="1"
          max="1440"
          class="input h-7 w-full rounded-lg px-1.5 py-0 text-xs"
          data-test="public-quality-cooldown"
        />
      </label>
      <label
        class="flex min-w-0 flex-col justify-end gap-1"
        :title="t('admin.accounts.publicQuality.softCooldown')"
      >
        <span class="truncate text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
          {{ t('admin.accounts.publicQuality.softCooldownShort') }}
        </span>
        <Toggle v-model="form.soft_cooldown" data-test="public-quality-soft" />
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { PublicScheduleQualitySettings } from '@/api/admin/settings'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  optionalNumber,
  percentToSuccessRate,
  successRateToPercent
} from '@/utils/accountQualityHardClose'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const loaded = ref<PublicScheduleQualitySettings | null>(null)

const form = reactive({
  enabled: false,
  ttft_window_n: '' as number | string,
  success_window_n: '' as number | string,
  quality_max_p50_ttft_ms: '' as number | string,
  min_success_rate_percent: '' as number | string,
  cooldown_minutes: '' as number | string,
  soft_cooldown: false
})

function applySettings(settings: PublicScheduleQualitySettings) {
  loaded.value = settings
  form.enabled = settings.enabled === true
  form.ttft_window_n = settings.ttft_window_n
  form.success_window_n = settings.success_window_n
  form.quality_max_p50_ttft_ms = settings.quality_max_p50_ttft_ms ?? ''
  form.min_success_rate_percent = successRateToPercent(settings.quality_min_success_rate) ?? ''
  form.cooldown_minutes = settings.cooldown_minutes
  form.soft_cooldown = settings.soft_cooldown === true
}

async function load() {
  loading.value = true
  try {
    applySettings(await adminAPI.settings.getPublicScheduleQualitySettings())
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.publicQuality.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const current = loaded.value ?? (await adminAPI.settings.getPublicScheduleQualitySettings())
    const updated = await adminAPI.settings.updatePublicScheduleQualitySettings({
      ...current,
      enabled: form.enabled,
      ttft_window_n: optionalNumber(form.ttft_window_n) ?? current.ttft_window_n,
      success_window_n: optionalNumber(form.success_window_n) ?? current.success_window_n,
      quality_max_p50_ttft_ms: optionalNumber(form.quality_max_p50_ttft_ms),
      quality_min_success_rate: percentToSuccessRate(form.min_success_rate_percent),
      cooldown_minutes: optionalNumber(form.cooldown_minutes) ?? current.cooldown_minutes,
      soft_cooldown: form.soft_cooldown
    })
    applySettings(updated)
    appStore.showSuccess(t('admin.accounts.publicQuality.saved'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.publicQuality.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
