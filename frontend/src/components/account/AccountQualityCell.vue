<template>
  <component
    :is="clickable ? 'button' : 'div'"
    :type="clickable ? 'button' : undefined"
    class="min-w-[7rem]"
    :class="clickable ? 'w-full rounded-md px-0.5 py-0.5 text-left transition-colors hover:bg-gray-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700' : undefined"
    :data-test="cellTestId"
    :aria-label="clickable ? openAria : undefined"
    :title="clickable && !tooltipText ? clickToOpen : undefined"
    @click="onClick"
  >
    <div v-if="loading && !hasStats && !hasSchedulerScore" class="h-3 w-14 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    <div v-else-if="error && !hasStats && !hasSchedulerScore" class="text-xs text-red-500">—</div>
    <div
      v-else-if="mode === 'combined' && (hasTtft || hasDuration || hasFailoverRate || hasSchedulerScore)"
      class="flex flex-col gap-0.5 font-mono text-[11px] leading-4"
      :title="tooltipText"
    >
      <div v-if="hasTtft" class="flex items-baseline gap-1 whitespace-nowrap" data-test="quality-ttft">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.ttftShort') }}</span>
        <span class="text-sm font-medium" data-test="quality-p95">
          <span data-test="quality-ttft-p50" :class="ttftP50Tone">{{ ttftP50Text }}</span><span class="text-gray-300 dark:text-gray-600">/</span><span data-test="quality-ttft-p95" :class="ttftP95Tone">{{ ttftP95Text }}</span>
        </span>
      </div>
      <div v-if="hasDuration" class="flex items-baseline gap-1 whitespace-nowrap" data-test="quality-duration">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.durationPairShort') }}</span>
        <span class="text-sm font-medium">
          <span data-test="quality-duration-p50" :class="durationP50Tone">{{ durationP50Text }}</span><span class="text-gray-300 dark:text-gray-600">/</span><span data-test="quality-duration-p95" :class="durationP95Tone">{{ durationP95Text }}</span>
        </span>
      </div>
      <div
        class="flex items-baseline gap-1"
        :class="failoverToneClass"
        data-test="account-quality-failover-rate"
      >
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.successShort') }}</span>
        <span class="text-sm font-medium">{{ failoverDisplay || '—' }}</span>
      </div>
      <div v-if="showLatencyKC && hasStats" class="font-sans text-[10px] text-gray-400 dark:text-gray-500" data-test="account-quality-window-counts">
        {{ t('admin.accounts.quality.windowCounts', { ttft: stats!.ttft_samples ?? 0, ok: okSamples, n: windowN }) }}
      </div>
      <div
        v-if="showLatencyKC && kc.show"
        class="font-sans text-[10px]"
        :class="kcToneClass"
        data-test="account-quality-kc"
      >
        <template v-if="kc.showK && kc.showC">
          {{ t('admin.users.smartSchedule.pairLatencyKC', { slow: kc.slow, k: kc.k, consec: kc.consec, c: kc.c }) }}
        </template>
        <template v-else-if="kc.showK">
          {{ t('admin.users.smartSchedule.pairLatencyK', { slow: kc.slow, k: kc.k }) }}
        </template>
        <template v-else>
          {{ t('admin.users.smartSchedule.pairLatencyC', { consec: kc.consec, c: kc.c }) }}
        </template>
      </div>
      <div
        v-for="(score, index) in visibleSchedulerScores"
        :key="`${score.group_id ?? 'ungrouped'}-${index}`"
        class="flex items-baseline gap-1 whitespace-nowrap text-gray-700 dark:text-gray-300"
        data-test="account-quality-scheduler-score"
      >
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.scoreShort') }}</span>
        <span v-if="schedulerScoreLabel(score)" class="max-w-[4.5rem] truncate text-[10px] font-sans text-gray-400">{{ schedulerScoreLabel(score) }}</span>
        <span class="text-sm font-medium">{{ formatSchedulerBase(score.base_score) }}</span>
      </div>
    </div>
    <div
      v-else-if="mode === 'ttft' && hasTtft"
      class="flex flex-col gap-0.5 font-mono text-[11px] leading-4"
      :title="tooltipText"
    >
      <div class="flex items-baseline gap-1 whitespace-nowrap">
        <span class="text-[10px] font-sans text-gray-400 dark:text-gray-500">{{ t('admin.accounts.quality.ttftShort') }}</span>
        <span class="text-sm font-medium">
          <span :class="ttftP50Tone">{{ ttftP50Text }}</span><span class="text-gray-300 dark:text-gray-600">/</span><span :class="ttftP95Tone">{{ ttftP95Text }}</span>
        </span>
      </div>
    </div>
    <div
      v-else-if="mode === 'success_rate' && displayText"
      class="text-sm font-mono"
      :class="successToneClass"
      :title="tooltipText"
    >
      {{ displayText }}
    </div>
    <div v-else class="flex flex-col items-start gap-0.5">
      <span class="text-sm text-gray-400 dark:text-dark-500">—</span>
      <span
        v-if="clickable"
        class="text-[10px] font-medium text-primary-600 dark:text-primary-400"
      >
        {{ openShort }}
      </span>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type { AccountSchedulerGroupScore, AccountSchedulerScore } from '@/types'
import {
  displayedFailoverSuccessRateValue,
  formatDisplayedFailoverSuccessRate,
  formatQualitySuccessRate,
  hasDisplayableQualityRate
} from '@/utils/accountQualityStats'
import { qualityRateWindowK, resolveAccountQualityWindowN } from '@/utils/accountQualityWindowN'
import { pairQualityLatencyKCParams } from '@/utils/smartScheduleWindowN'
import { durationSeverity, firstTokenSeverity, LATENCY_TEXT_CLASSES } from '@/utils/latencyHealth'

const props = withDefaults(
  defineProps<{
    mode: 'ttft' | 'success_rate' | 'combined'
    stats?: AccountQualityStats | null
    loading?: boolean
    error?: string | null
    clickable?: boolean
    minSamples?: number
    windowN?: number | null
    subject?: 'account' | 'user'
    schedulerScore?: AccountSchedulerScore | null
    schedulerScores?: AccountSchedulerGroupScore[] | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null,
    clickable: false,
    minSamples: 1,
    windowN: null,
    subject: 'account',
    schedulerScore: null,
    schedulerScores: null
  }
)

// k/N 与 K/C 先不画。首字 p50/p95、总耗时、正确率、普通账号池调度分留下。
const showLatencyKC = false

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()

const isUserSubject = computed(() => props.subject === 'user')

const cellTestId = computed(() => {
  if (props.clickable) {
    return isUserSubject.value ? 'user-quality-cell-button' : 'account-quality-cell-button'
  }
  return isUserSubject.value ? 'user-quality-cell' : 'account-quality-cell'
})

const openAria = computed(() =>
  isUserSubject.value ? t('admin.users.quality.openAria') : t('admin.accounts.stability.openAria')
)

const clickToOpen = computed(() =>
  isUserSubject.value ? t('admin.users.quality.clickToOpen') : t('admin.accounts.stability.clickToOpen')
)

const openShort = computed(() =>
  isUserSubject.value ? t('admin.users.quality.openShort') : t('admin.accounts.stability.openShort')
)

const hasStats = computed(() => props.stats != null)

const windowN = computed(() =>
  resolveAccountQualityWindowN({
    account_quality_window_n: props.stats?.account_quality_window_n ?? props.windowN,
    window_n: props.stats?.window_n,
    n: props.stats?.n
  })
)

const okSamples = computed(() => qualityRateWindowK(props.stats))

const hasTtft = computed(() => {
  const s = props.stats
  if (!s || !s.ttft_samples) return false
  return s.p50_ttft_ms != null || s.avg_ttft_ms != null
})

const hasDuration = computed(() => {
  const s = props.stats
  if (!s || !(s.duration_samples ?? 0)) return false
  return s.p50_duration_ms != null || s.p95_duration_ms != null
})

const visibleSchedulerScores = computed((): AccountSchedulerGroupScore[] => {
  if (props.subject !== 'account') return []
  const listed = props.schedulerScores?.length
    ? props.schedulerScores
    : props.schedulerScore
      ? [{ ...props.schedulerScore, group_id: null }]
      : []
  return listed.filter((score) => score != null && Number.isFinite(score.base_score))
})

const hasSchedulerScore = computed(() => visibleSchedulerScores.value.length > 0)

function formatSchedulerBase(score: number): string {
  return score.toFixed(2)
}

function schedulerScoreLabel(score: AccountSchedulerGroupScore): string {
  if (score.group_name) return score.group_name
  if (visibleSchedulerScores.value.length > 1 && score.group_id == null) {
    return t('admin.accounts.schedulerScore.ungrouped')
  }
  return ''
}

function formatPairSeconds(ms: number | null | undefined, digits: 0 | 1): string {
  if (ms == null || !Number.isFinite(ms)) return '—'
  const seconds = ms / 1000
  if (digits === 0) return `${Math.round(seconds)}s`
  return `${seconds.toFixed(1)}s`
}

function latencyTone(ms: number | null | undefined, kind: 'ttft' | 'duration'): string {
  if (ms == null || !Number.isFinite(ms)) return 'text-gray-400 dark:text-gray-500'
  const severity = kind === 'ttft' ? firstTokenSeverity(ms) : durationSeverity(ms)
  return LATENCY_TEXT_CLASSES[severity]
}

const hasSuccessRate = computed(() => hasDisplayableQualityRate(props.stats, props.minSamples))

const successDisplay = computed(() => formatQualitySuccessRate(props.stats, props.minSamples) ?? '')

const failoverSuccessRate = computed(() =>
  displayedFailoverSuccessRateValue(props.stats, props.minSamples)
)

const failoverDisplay = computed(
  () => formatDisplayedFailoverSuccessRate(props.stats, props.minSamples) ?? ''
)

const hasFailoverRate = computed(() => failoverDisplay.value !== '')

const kc = computed(() =>
  props.stats
    ? pairQualityLatencyKCParams(props.stats)
    : { show: false, showK: false, showC: false, slow: 0, consec: 0, k: 0, c: 0 }
)

const kcToneClass = computed(() => {
  const row = kc.value
  if (!row.show) return 'text-gray-400 dark:text-gray-500'
  if ((row.showK && row.slow >= row.k) || (row.showC && row.consec >= row.c)) {
    return 'text-red-600 dark:text-red-400'
  }
  return 'text-gray-400 dark:text-gray-500'
})

const displayText = computed(() => {
  if (props.mode !== 'success_rate') return ''
  return successDisplay.value
})

const ttftP50Ms = computed(() => props.stats?.p50_ttft_ms ?? props.stats?.avg_ttft_ms)
const ttftP50Text = computed(() => formatPairSeconds(ttftP50Ms.value, 1))
const ttftP95Text = computed(() => formatPairSeconds(props.stats?.p95_ttft_ms, 0))
const durationP50Text = computed(() => formatPairSeconds(props.stats?.p50_duration_ms, 1))
const durationP95Text = computed(() => formatPairSeconds(props.stats?.p95_duration_ms, 0))
const ttftP50Tone = computed(() => latencyTone(ttftP50Ms.value, 'ttft'))
const ttftP95Tone = computed(() => latencyTone(props.stats?.p95_ttft_ms, 'ttft'))
const durationP50Tone = computed(() => latencyTone(props.stats?.p50_duration_ms, 'duration'))
const durationP95Tone = computed(() => latencyTone(props.stats?.p95_duration_ms, 'duration'))

const successToneClass = computed(() => {
  if (!hasSuccessRate.value) return 'text-gray-700 dark:text-gray-300'
  const rate = props.stats?.success_rate
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const failoverToneClass = computed(() => {
  const rate = failoverSuccessRate.value
  if (rate == null) return 'text-gray-700 dark:text-gray-300'
  if (rate < 0.9) return 'text-red-600 dark:text-red-400'
  if (rate < 0.95) return 'text-amber-600 dark:text-amber-400'
  return 'text-emerald-600 dark:text-emerald-400'
})

const schedulerScoreTooltip = computed(() => {
  const score = visibleSchedulerScores.value[0]
  const factors = score?.factors
  if (!score) return ''
  return t('admin.accounts.quality.scoreTooltip', {
    score: formatSchedulerBase(score.base_score),
    priority: factors?.priority_term?.toFixed(2) ?? '—',
    load: factors?.load_term?.toFixed(2) ?? '—',
    queue: factors?.queue_term?.toFixed(2) ?? '—',
    error: factors?.error_term?.toFixed(2) ?? '—',
    errorRate: factors ? (factors.error_rate * 100).toFixed(1) : '—',
    ttft: factors?.ttft_term?.toFixed(2) ?? '—',
    ttftMs: factors?.has_ttft ? Math.round(factors.ttft_ms) : '—',
    reset: factors?.reset_term?.toFixed(2) ?? '—',
    quota: factors?.quota_term?.toFixed(2) ?? '—',
    previous: factors?.weight_previous?.toFixed(2) ?? '—',
    session: factors?.weight_session_sticky?.toFixed(2) ?? '—'
  })
})

const tooltipText = computed(() => {
  const stats = props.stats
  const clickHint = props.clickable ? clickToOpen.value : ''
  if (!stats) {
    const scoreOnly = schedulerScoreTooltip.value
    if (!scoreOnly) return clickHint
    return clickHint ? `${scoreOnly}\n${clickHint}` : scoreOnly
  }
  let base = ''
  if (props.mode === 'combined') {
    const parts: string[] = []
    if (hasTtft.value) {
      parts.push(
        t('admin.accounts.quality.ttftTooltip', {
          n: windowN.value,
          samples: stats.ttft_samples ?? 0,
          p50: stats.p50_ttft_ms ?? '—',
          p95: stats.p95_ttft_ms ?? '—',
          avg: stats.avg_ttft_ms ?? '—',
          max: stats.max_ttft_ms ?? '—'
        })
      )
    }
    if (hasFailoverRate.value) {
      const displaySamples = (stats.display_success_count ?? 0) + (stats.display_failover_error_count ?? 0)
      parts.push(
        displaySamples > 0 && stats.display_rate_mode === 'count'
          ? t('admin.accounts.quality.displayRateCountTooltip', {
              count: stats.display_rate_window_count ?? 0,
              success: stats.display_success_count ?? 0,
              error: stats.display_failover_error_count ?? 0
            })
          : displaySamples > 0
          ? t('admin.accounts.quality.displayRateTooltip', {
              minutes: Math.max(1, Math.round((stats.display_rate_window_seconds ?? 300) / 60)),
              success: stats.display_success_count ?? 0,
              error: stats.display_failover_error_count ?? 0
            })
          : t('admin.accounts.quality.failoverTooltip', {
              n: windowN.value,
              error: stats.failover_error_count ?? '—'
            })
      )
    }
    if (hasDuration.value) {
      parts.push(
        t('admin.accounts.quality.durationTooltip', {
          n: windowN.value,
          samples: stats.duration_samples ?? 0,
          p50: stats.p50_duration_ms ?? '—',
          p95: stats.p95_duration_ms ?? '—'
        })
      )
    }
    if (hasSchedulerScore.value) parts.push(schedulerScoreTooltip.value)
    base = parts.join('\n')
  } else if (props.mode === 'ttft') {
    base = t('admin.accounts.quality.ttftTooltip', {
      n: windowN.value,
      samples: stats.ttft_samples ?? 0,
      p50: stats.p50_ttft_ms ?? '—',
      p95: stats.p95_ttft_ms ?? '—',
      avg: stats.avg_ttft_ms ?? '—',
      max: stats.max_ttft_ms ?? '—'
    })
  } else {
    base = t('admin.accounts.quality.tooltip', {
      n: windowN.value,
      success: stats.success_count ?? 0,
      error: stats.error_count ?? 0,
      ttftSamples: stats.ttft_samples ?? 0
    })
  }
  return clickHint ? `${base}\n${clickHint}` : base
})

function onClick(event: Event) {
  if (!props.clickable) return
  event.stopPropagation()
  emit('click')
}
</script>
