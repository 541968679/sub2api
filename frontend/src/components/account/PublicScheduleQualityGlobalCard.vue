<template>
  <div
    class="flex h-full min-h-0 w-full flex-col gap-1.5 overflow-hidden rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 shadow-sm dark:border-amber-700/40 dark:bg-amber-900/20"
    data-test="public-schedule-quality-global"
  >
    <div class="flex shrink-0 items-center gap-2">
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

    <div
      v-else
      class="flex min-h-0 min-w-0 flex-1 flex-col gap-1.5 overflow-hidden"
      data-test="public-quality-knob-strip"
    >
      <div
        class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-amber-200/80 px-2 py-1 dark:border-amber-700/50"
        data-test="public-quality-top-bar"
      >
        <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1" data-test="public-quality-threshold-group">
          <span class="shrink-0 text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
            {{ t('admin.users.smartSchedule.thresholdMsGroup') }}
          </span>
          <label class="flex min-w-0 items-center gap-1">
            <span class="shrink-0 text-[11px] text-amber-900/80 dark:text-amber-100/80">
              {{ t('admin.accounts.publicQuality.maxP50Short') }}
            </span>
            <input
              v-model="form.quality_max_p50_ttft_ms"
              type="number"
              min="1"
              class="input h-7 w-16 rounded-lg px-1.5 py-0 text-xs"
              data-test="public-quality-p50"
            />
          </label>
          <label class="flex min-w-0 items-center gap-1">
            <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
              {{ t('admin.accounts.publicQuality.durationShort') }}
              <HelpTooltip :content="t('admin.users.smartSchedule.maxP50DurationHint')" width-class="w-72" />
            </span>
            <input
              v-model="form.quality_max_p50_duration_ms"
              type="number"
              min="1"
              class="input h-7 w-16 rounded-lg px-1.5 py-0 text-xs"
              data-test="public-quality-p50-duration"
            />
          </label>
          <label class="flex min-w-0 items-center gap-1">
            <span class="shrink-0 text-[11px] text-amber-900/80 dark:text-amber-100/80">
              {{ t('admin.accounts.publicQuality.minSuccessRateShort') }}
            </span>
            <input
              v-model="form.min_success_rate_percent"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input h-7 w-16 rounded-lg px-1.5 py-0 text-xs"
              data-test="public-quality-success-rate"
            />
          </label>
        </div>

        <div
          class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 border-l border-amber-200/80 pl-3 dark:border-amber-700/50"
          data-test="public-quality-cooldown-row"
        >
          <span class="shrink-0 text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
            {{ t('admin.accounts.publicQuality.cooldownGroup') }}
          </span>
          <label class="flex min-w-0 items-center gap-1">
            <span class="shrink-0 text-[11px] text-amber-900/80 dark:text-amber-100/80">
              {{ t('admin.accounts.publicQuality.cooldownMinutes') }}
            </span>
            <input
              v-model="form.cooldown_minutes"
              type="number"
              min="1"
              max="1440"
              class="input h-7 w-14 rounded-lg px-1.5 py-0 text-xs"
              data-test="public-quality-cooldown"
            />
          </label>
          <div class="flex items-center gap-1">
            <span class="shrink-0 text-[11px] text-amber-900/80 dark:text-amber-100/80">
              {{ t('admin.accounts.publicQuality.softCooldownShort') }}
            </span>
            <div
              class="inline-flex overflow-hidden rounded-md border border-amber-300 dark:border-amber-700/60"
              data-test="public-quality-cooldown-mode"
            >
              <button
                type="button"
                class="px-2 py-0.5 text-xs"
                :class="
                  !form.soft_cooldown
                    ? 'bg-amber-800 text-white dark:bg-amber-200 dark:text-amber-950'
                    : 'bg-white text-amber-800 dark:bg-amber-950/40 dark:text-amber-200'
                "
                :aria-pressed="!form.soft_cooldown"
                data-test="public-quality-hard"
                @click="form.soft_cooldown = false"
              >
                {{ t('admin.users.smartSchedule.cooldownModeHard') }}
              </button>
              <button
                type="button"
                class="px-2 py-0.5 text-xs"
                :class="
                  form.soft_cooldown
                    ? 'bg-amber-800 text-white dark:bg-amber-200 dark:text-amber-950'
                    : 'bg-white text-amber-800 dark:bg-amber-950/40 dark:text-amber-200'
                "
                :aria-pressed="form.soft_cooldown"
                data-test="public-quality-soft"
                @click="form.soft_cooldown = true"
              >
                {{ t('admin.users.smartSchedule.cooldownModeSoft') }}
              </button>
            </div>
            <HelpTooltip
              :content="
                form.soft_cooldown
                  ? t('admin.accounts.publicQuality.softCooldown')
                  : t('admin.users.smartSchedule.cooldownModeHardHint')
              "
              width-class="w-72"
            />
          </div>
        </div>
      </div>

      <div class="grid min-h-0 min-w-0 flex-1 grid-cols-2 gap-1.5">
        <div
          class="flex min-h-0 min-w-0 flex-col rounded-lg border border-amber-200/80 px-2.5 py-1.5 dark:border-amber-700/50"
          data-test="public-quality-probe-group"
        >
          <p class="shrink-0 text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
            {{ t('admin.users.smartSchedule.probePhaseGroup') }}
          </p>
          <div class="grid min-h-0 flex-1 grid-cols-2 content-center gap-x-3 gap-y-1">
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.ttftNShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.windowNTtftHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.ttft_window_n"
                type="number"
                min="1"
                max="100"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-ttft-n"
              />
            </label>
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.successNShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.windowNSuccessHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.success_window_n"
                type="number"
                min="1"
                max="100"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-success-n"
              />
            </label>
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.kShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.maxSlowInWindowHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.quality_max_slow_in_window"
                type="number"
                min="1"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-probe-k"
              />
            </label>
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.cShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.maxConsecutiveSlowHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.quality_max_consecutive_slow"
                type="number"
                min="1"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-probe-c"
              />
            </label>
          </div>
        </div>

        <div
          class="flex min-h-0 min-w-0 flex-col rounded-lg border border-amber-200/80 px-2.5 py-1.5 dark:border-amber-700/50"
          data-test="public-quality-sched-group"
        >
          <p class="shrink-0 text-[11px] font-medium leading-4 text-amber-900 dark:text-amber-100">
            {{ t('admin.users.smartSchedule.schedPhaseGroup') }}
          </p>
          <div class="grid min-h-0 flex-1 grid-cols-3 content-center gap-x-3">
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.nShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.schedWindowNHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.quality_sched_window_n"
                type="number"
                min="1"
                max="100"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-sched-n"
              />
            </label>
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.kShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.schedMaxSlowInWindowHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.quality_sched_max_slow_in_window"
                type="number"
                min="1"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-sched-k"
              />
            </label>
            <label class="flex min-w-0 items-center gap-1">
              <span class="inline-flex shrink-0 items-center gap-0.5 text-[11px] text-amber-900/80 dark:text-amber-100/80">
                {{ t('admin.accounts.publicQuality.cShort') }}
                <HelpTooltip :content="t('admin.users.smartSchedule.schedMaxConsecutiveSlowHint')" width-class="w-72" />
              </span>
              <input
                v-model="form.quality_sched_max_consecutive_slow"
                type="number"
                min="1"
                class="input h-7 min-w-0 flex-1 rounded-lg px-1.5 py-0 text-xs"
                data-test="public-quality-sched-c"
              />
            </label>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { PublicScheduleQualitySettings } from '@/api/admin/settings'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
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
  quality_max_p50_duration_ms: '' as number | string,
  min_success_rate_percent: '' as number | string,
  quality_max_slow_in_window: '' as number | string,
  quality_max_consecutive_slow: '' as number | string,
  quality_sched_window_n: '' as number | string,
  quality_sched_max_slow_in_window: '' as number | string,
  quality_sched_max_consecutive_slow: '' as number | string,
  cooldown_minutes: '' as number | string,
  soft_cooldown: false
})

function emptyable(value: number | null | undefined): number | string {
  return value == null ? '' : value
}

function applySettings(settings: PublicScheduleQualitySettings) {
  loaded.value = settings
  form.enabled = settings.enabled === true
  form.ttft_window_n = settings.ttft_window_n
  form.success_window_n = settings.success_window_n
  form.quality_max_p50_ttft_ms = emptyable(settings.quality_max_p50_ttft_ms)
  form.quality_max_p50_duration_ms = emptyable(settings.quality_max_p50_duration_ms)
  form.min_success_rate_percent = successRateToPercent(settings.quality_min_success_rate) ?? ''
  form.quality_max_slow_in_window = emptyable(settings.quality_max_slow_in_window)
  form.quality_max_consecutive_slow = emptyable(settings.quality_max_consecutive_slow)
  form.quality_sched_window_n = emptyable(settings.quality_sched_window_n)
  form.quality_sched_max_slow_in_window = emptyable(settings.quality_sched_max_slow_in_window)
  form.quality_sched_max_consecutive_slow = emptyable(settings.quality_sched_max_consecutive_slow)
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
      quality_max_p50_duration_ms: optionalNumber(form.quality_max_p50_duration_ms),
      quality_min_success_rate: percentToSuccessRate(form.min_success_rate_percent),
      quality_max_slow_in_window: optionalNumber(form.quality_max_slow_in_window),
      quality_max_consecutive_slow: optionalNumber(form.quality_max_consecutive_slow),
      quality_sched_window_n: optionalNumber(form.quality_sched_window_n),
      quality_sched_max_slow_in_window: optionalNumber(form.quality_sched_max_slow_in_window),
      quality_sched_max_consecutive_slow: optionalNumber(form.quality_sched_max_consecutive_slow),
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
