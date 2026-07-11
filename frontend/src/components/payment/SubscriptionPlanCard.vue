<template>
  <div
    :class="[
      'group relative flex min-h-[224px] flex-col overflow-hidden rounded-lg border transition-all',
      'hover:-translate-y-0.5 hover:shadow-lg',
      borderClass,
      'bg-white dark:bg-dark-800',
    ]"
  >
    <!-- Colored top accent bar -->
    <div :class="['h-1.5', accentClass]" />

    <div class="flex flex-1 flex-col p-4">
      <!-- Header: name + price -->
      <div class="mb-3 flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <h3 class="line-clamp-1 text-base font-bold text-gray-900 dark:text-white">{{ plan.name }}</h3>
          <p v-if="plan.description" class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400 line-clamp-2">
            {{ plan.description }}
          </p>
        </div>
        <div class="shrink-0 text-right">
          <div class="flex items-baseline justify-end gap-1">
            <span class="text-xs text-gray-400 dark:text-dark-500">¥</span>
            <span :class="['text-2xl font-extrabold', textClass]">{{ plan.price }}</span>
          </div>
          <span class="text-[11px] text-gray-400 dark:text-dark-500">/ {{ validitySuffix }}</span>
          <div v-if="plan.original_price" class="mt-1 flex items-center justify-end gap-1">
            <span class="text-[11px] text-gray-400 line-through dark:text-dark-500">¥{{ plan.original_price }}</span>
            <span :class="['rounded px-2 py-0.5 text-[10px] font-semibold', discountClass]">{{ discountText }}</span>
          </div>
        </div>
      </div>

      <!-- Bundle: included groups, each with its own independent quota pool -->
      <div v-if="isBundle" class="mb-3 space-y-1.5 rounded-lg bg-gray-50 p-3 text-xs dark:bg-dark-700/50">
        <span class="block text-[11px] font-medium text-gray-400 dark:text-dark-500">{{ t('payment.planCard.includedGroups') }}</span>
        <div v-for="mg in plan.member_groups" :key="mg.group_id" class="flex items-center justify-between gap-2">
          <span class="truncate font-semibold" :class="platformTextClass(mg.platform || '')">{{ mg.name || ('#' + mg.group_id) }}</span>
          <span class="shrink-0 text-[11px] text-gray-500 dark:text-dark-400">{{ memberQuotaText(mg) }}</span>
        </div>
        <p class="pt-1 text-[10px] leading-4 text-gray-400 dark:text-dark-500">{{ t('payment.planCard.bundleQuotaNote') }}</p>
      </div>

      <!-- Group quota info (single-group plans) -->
      <div v-else class="mb-3 grid grid-cols-3 gap-2 rounded-lg bg-gray-50 p-3 text-xs dark:bg-dark-700/50">
        <div v-if="hasPeakRate" class="col-span-3 flex items-center justify-between gap-2">
          <span class="text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
          <span class="text-right font-medium text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</span>
        </div>
        <div v-if="plan.daily_limit_usd != null" class="space-y-1">
          <span class="block truncate text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
          <p class="truncate font-semibold text-gray-700 dark:text-gray-300">${{ plan.daily_limit_usd }}</p>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="space-y-1">
          <span class="block truncate text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
          <p class="truncate font-semibold text-gray-700 dark:text-gray-300">${{ plan.weekly_limit_usd }}</p>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="space-y-1">
          <span class="block truncate text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
          <p class="truncate font-semibold text-gray-700 dark:text-gray-300">${{ plan.monthly_limit_usd }}</p>
        </div>
        <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="space-y-1">
          <span class="block truncate text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
          <p class="truncate font-semibold text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</p>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="col-span-3 flex items-center gap-1.5 overflow-hidden">
          <span class="shrink-0 text-[11px] text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
          <div class="flex min-w-0 flex-wrap gap-1">
            <span v-for="scope in modelScopeLabels" :key="scope"
              class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ scope }}
            </span>
          </div>
        </div>
      </div>

      <!-- Features list -->
      <div v-if="compactFeatures.length > 0" class="mb-3 space-y-1">
        <div v-for="feature in compactFeatures" :key="feature" class="flex items-start gap-1.5">
          <svg :class="['mt-0.5 h-3.5 w-3.5 flex-shrink-0', iconClass]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span class="line-clamp-1 text-xs text-gray-600 dark:text-gray-300">{{ feature }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <!-- Subscribe Button -->
      <button
        type="button"
        :class="['w-full rounded-lg py-2.5 text-sm font-semibold transition-all active:scale-[0.98]', btnClass]"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan, PlanMemberGroup } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import {
  platformAccentBarClass,
  platformBorderClass,
  platformTextClass,
  platformIconClass,
  platformButtonClass,
  platformDiscountClass,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()
const appStore = useAppStore()

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))
const peakRateDisplay = computed(() =>
  formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
)

const platform = computed(() => props.plan.group_platform || '')
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

// A bundle plan bundles more than one group; render each member's own quota pool.
const isBundle = computed(() => (props.plan.member_groups?.length ?? 0) > 1)

function memberQuotaText(mg: PlanMemberGroup): string {
  if (mg.daily_limit_usd != null) return `${t('payment.planCard.dailyLimit')} $${mg.daily_limit_usd}`
  if (mg.weekly_limit_usd != null) return `${t('payment.planCard.weeklyLimit')} $${mg.weekly_limit_usd}`
  if (mg.monthly_limit_usd != null) return `${t('payment.planCard.monthlyLimit')} $${mg.monthly_limit_usd}`
  return t('payment.planCard.unlimited')
}

// Derived color classes from central config
const accentClass = computed(() => platformAccentBarClass(platform.value))
const borderClass = computed(() => platformBorderClass(platform.value))
const textClass = computed(() => platformTextClass(platform.value))
const iconClass = computed(() => platformIconClass(platform.value))
const btnClass = computed(() => platformButtonClass(platform.value))
const discountClass = computed(() => platformDiscountClass(platform.value))

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const ALL_MODEL_SCOPES = Object.keys(MODEL_SCOPE_LABELS)

const modelScopeLabels = computed(() => {
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  // If all scopes are included, it means "no restriction" — hide the row
  if (scopes.length >= ALL_MODEL_SCOPES.length && ALL_MODEL_SCOPES.every(s => scopes.includes(s))) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const compactFeatures = computed(() => props.plan.features.slice(0, 2))

const validitySuffix = computed(() => {
  const u = props.plan.validity_unit || 'day'
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  return `${props.plan.validity_days}${t('payment.days')}`
})
</script>
