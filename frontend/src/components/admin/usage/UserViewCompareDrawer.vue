<template>
  <BaseDialog :show="open" :title="t('admin.usage.userViewCompareTitle')" width="extra-wide" close-on-click-outside @close="$emit('close')">
    <div v-if="loading" class="py-12 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="errorMsg" class="py-8 text-center text-sm text-red-600 dark:text-red-400">
      {{ errorMsg }}
    </div>
    <div v-else-if="preview" class="space-y-5">
      <!-- Summary header -->
      <div class="flex flex-wrap items-baseline gap-3 rounded-md bg-gray-50 px-3 py-2 text-xs dark:bg-dark-800">
        <span class="font-mono text-gray-700 dark:text-gray-300">log #{{ preview.log_id }}</span>
        <span class="text-gray-400 dark:text-gray-500">·</span>
        <span class="text-gray-700 dark:text-gray-300">user #{{ preview.user_id }}</span>
        <span class="text-gray-400 dark:text-gray-500">·</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ preview.model }}</span>
      </div>

      <!-- Config used -->
      <div class="rounded-md border border-gray-200 p-3 text-xs dark:border-dark-700">
        <div class="mb-2 flex items-center gap-2 text-sm font-medium text-gray-800 dark:text-gray-200">
          <Icon name="cog" size="sm" />
          {{ t('admin.usage.userViewConfigUsed') }}
          <span
            class="ml-auto rounded px-2 py-0.5 text-[10px] font-semibold"
            :class="preview.config_used.has_user_override
              ? 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
              : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'">
            {{ preview.config_used.has_user_override
              ? t('admin.usage.userViewSourceOverride')
              : t('admin.usage.userViewSourceGlobal') }}
          </span>
        </div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-gray-700 dark:text-gray-300 sm:grid-cols-3">
          <div v-for="cfg in configRows" :key="cfg.key" class="flex items-baseline justify-between gap-2 truncate">
            <span class="text-[11px] text-gray-500 dark:text-gray-400">{{ cfg.label }}</span>
            <span :class="cfg.value == null ? 'text-gray-400 dark:text-gray-600' : 'font-mono text-gray-800 dark:text-gray-200'">
              {{ formatConfig(cfg.value, cfg.format) }}
            </span>
          </div>
        </div>
        <p class="mt-2 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
          {{ t('admin.usage.userViewConfigHint') }}
        </p>
      </div>

      <!-- Comparison sections -->
      <div v-for="section in sections" :key="section.title" class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
        <div class="bg-gray-50 px-3 py-2 text-sm font-medium text-gray-800 dark:bg-dark-800 dark:text-gray-200">
          {{ section.title }}
        </div>
        <table v-if="section.rows.length" class="w-full text-xs">
          <thead class="bg-gray-50/40 dark:bg-dark-800/60">
            <tr>
              <th class="px-3 py-1.5 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewField') }}</th>
              <th class="px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ realLabel }}</th>
              <th class="px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ userLabel }}</th>
              <th class="w-20 px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewDiff') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="r in section.rows" :key="r.key">
              <td class="px-3 py-1.5 text-gray-700 dark:text-gray-300">{{ r.label }}</td>
              <td class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-white">{{ formatCell(r.real, r.format) }}</td>
              <td class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-white">{{ formatCell(r.user, r.format) }}</td>
              <td class="px-3 py-1.5 text-right font-mono" :class="diffClass(r.real, r.user)">{{ diffText(r.real, r.user) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="px-3 py-2 text-xs text-gray-400 dark:text-gray-600">{{ t('admin.usage.userViewEmptySection') }}</p>
      </div>

      <!-- Cost calculation process -->
      <div class="space-y-3">
        <div>
          <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.usage.userViewCalculationTitle') }}</div>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewCalculationHint') }}</p>
        </div>
        <div class="grid gap-4 xl:grid-cols-2">
          <div
            v-for="calc in calculationPanels"
            :key="calc.key"
            class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700"
          >
            <div class="flex items-center justify-between bg-gray-50 px-3 py-2 dark:bg-dark-800">
              <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ calc.title }}</span>
              <span class="font-mono text-xs text-gray-500 dark:text-gray-400">{{ formatRate(calc.snapshot.rate_multiplier) }}</span>
            </div>
            <table class="w-full text-xs">
              <thead class="bg-gray-50/40 dark:bg-dark-800/60">
                <tr>
                  <th class="px-3 py-1.5 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaItem') }}</th>
                  <th class="px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaTokens') }}</th>
                  <th class="px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaUnitPrice') }}</th>
                  <th class="px-3 py-1.5 text-right font-medium text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaCost') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr v-for="row in calc.rows" :key="row.key">
                  <td class="px-3 py-1.5 text-gray-700 dark:text-gray-300">
                    <div>{{ row.label }}</div>
                    <div class="font-mono text-[11px] text-gray-400 dark:text-gray-500">{{ row.formula }}</div>
                  </td>
                  <td class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-white">{{ formatFormulaTokens(row.tokens) }}</td>
                  <td class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-white">{{ formatUnitPrice(row.unitPrice) }}</td>
                  <td class="px-3 py-1.5 text-right font-mono text-gray-900 dark:text-white">{{ formatUsd(row.cost) }}</td>
                </tr>
              </tbody>
            </table>
            <div class="space-y-1 border-t border-gray-100 bg-gray-50/50 px-3 py-2 text-xs dark:border-dark-700 dark:bg-dark-800/50">
              <div class="flex items-center justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaSubtotal') }}</span>
                <span class="font-mono text-gray-900 dark:text-white">{{ formatUsd(calc.componentSubtotal) }}</span>
              </div>
              <div v-if="calc.hasOtherCost" class="flex items-center justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaOther') }}</span>
                <span class="font-mono text-gray-900 dark:text-white">{{ formatUsd(calc.otherCost) }}</span>
              </div>
              <div class="flex items-center justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaTotal') }}</span>
                <span class="font-mono text-gray-900 dark:text-white">{{ formatUsd(calc.snapshot.total_cost) }}</span>
              </div>
              <div class="flex items-center justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaBilled') }}</span>
                <span class="font-mono text-gray-900 dark:text-white">{{ formatUsd(calc.billedCost) }}</span>
              </div>
              <div class="flex items-center justify-between gap-3">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaActual') }}</span>
                <span class="font-mono text-gray-900 dark:text-white">{{ formatUsd(calc.snapshot.actual_cost) }}</span>
              </div>
              <div class="flex items-center justify-between gap-3 border-t border-gray-200 pt-1 dark:border-dark-700">
                <span class="text-gray-500 dark:text-gray-400">{{ t('admin.usage.userViewFormulaDiff') }}</span>
                <span class="font-mono font-medium" :class="calcDiffClass(calc.diff)">
                  {{ formatSignedUsd(calc.diff) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <p v-if="actualCostMismatch" class="rounded border border-red-300 bg-red-50 p-2 text-xs text-red-700 dark:border-red-700 dark:bg-red-950 dark:text-red-300">
        {{ t('admin.usage.userViewActualCostMismatch') }}
      </p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminUsageAPI, type UserViewPreview, type UserViewSnapshot } from '@/api/admin/usage'

interface Props {
  logId: number | null
  open: boolean
}

const props = defineProps<Props>()
defineEmits<{ close: [] }>()

const { t } = useI18n()

const loading = ref(false)
const errorMsg = ref('')
const preview = ref<UserViewPreview | null>(null)

const realLabel = computed(() => t('admin.usage.userViewColReal'))
const userLabel = computed(() => t('admin.usage.userViewColUser'))

watch(
  () => [props.open, props.logId] as const,
  async ([open, id]) => {
    if (!open || !id) {
      preview.value = null
      errorMsg.value = ''
      return
    }
    loading.value = true
    errorMsg.value = ''
    preview.value = null
    try {
      preview.value = await adminUsageAPI.getUserViewPreview(id)
    } catch (e: any) {
      errorMsg.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)

type CellFormat = 'int' | 'usd' | 'rate'
type ConfigFormat = 'price' | 'ratio' | 'rate'
type FormulaMode = 'real' | 'user'

interface CmpRow { key: string; label: string; real: number; user: number; format: CellFormat }
interface CfgRow { key: string; label: string; value: number | null; format: ConfigFormat }
interface FormulaRow {
  key: string
  label: string
  tokens: number | null
  unitPrice: number | null
  formula: string
  cost: number
}
interface CalculationPanel {
  key: FormulaMode
  title: string
  snapshot: UserViewSnapshot
  rows: FormulaRow[]
  componentSubtotal: number
  otherCost: number
  hasOtherCost: boolean
  billedCost: number
  diff: number
}

const epsilon = 1e-9

const buildRow = (key: string, label: string, getter: (s: UserViewSnapshot) => number, format: CellFormat, snap: UserViewPreview): CmpRow => ({
  key, label, real: getter(snap.real), user: getter(snap.user_view), format
})

const filterNonZero = (rows: CmpRow[]): CmpRow[] => rows.filter(r => r.real !== 0 || r.user !== 0)

const sections = computed(() => {
  if (!preview.value) return []
  const p = preview.value
  return [
    {
      title: t('admin.usage.userViewSectionTokens'),
      rows: filterNonZero([
        buildRow('input_tokens', t('usage.input'), s => s.input_tokens, 'int', p),
        buildRow('cache_read_tokens', t('usage.cacheRead'), s => s.cache_read_tokens, 'int', p),
        buildRow('cache_creation_tokens', t('usage.cacheCreation'), s => s.cache_creation_tokens, 'int', p),
        buildRow('output_tokens', t('usage.output'), s => s.output_tokens, 'int', p)
      ])
    },
    {
      title: t('admin.usage.userViewSectionCosts'),
      rows: filterNonZero([
        buildRow('input_cost', t('usage.input'), s => s.input_cost, 'usd', p),
        buildRow('cache_read_cost', t('usage.cacheRead'), s => s.cache_read_cost, 'usd', p),
        buildRow('cache_creation_cost', t('usage.cacheCreation'), s => s.cache_creation_cost, 'usd', p),
        buildRow('output_cost', t('usage.output'), s => s.output_cost, 'usd', p),
        buildRow('total_cost', t('admin.usage.userViewTotal'), s => s.total_cost, 'usd', p)
      ])
    },
    {
      title: t('admin.usage.userViewSectionInvariants'),
      rows: [
        buildRow('actual_cost', t('admin.usage.userViewActualCost'), s => s.actual_cost, 'usd', p),
        buildRow('rate_multiplier', t('admin.usage.userViewRateMultiplier'), s => s.rate_multiplier, 'rate', p)
      ]
    }
  ]
})

const configRows = computed<CfgRow[]>(() => {
  if (!preview.value) return []
  const c = preview.value.config_used
  return [
    { key: 'display_input_price', label: t('admin.modelPricing.displayInputPrice'), value: c.display_input_price, format: 'price' },
    { key: 'display_output_price', label: t('admin.modelPricing.displayOutputPrice'), value: c.display_output_price, format: 'price' },
    { key: 'display_cache_read_price', label: t('admin.modelPricing.displayCacheReadPrice'), value: c.display_cache_read_price, format: 'price' },
    { key: 'display_cache_creation_price', label: t('admin.modelPricing.displayCacheCreationPrice'), value: c.display_cache_creation_price, format: 'price' },
    { key: 'display_cache_creation_1h_price', label: t('admin.modelPricing.displayCacheCreation1hPrice'), value: c.display_cache_creation_1h_price, format: 'price' },
    { key: 'display_rate_multiplier', label: t('admin.modelPricing.displayRateMultiplier'), value: c.display_rate_multiplier, format: 'rate' },
    { key: 'user_group_rate', label: t('admin.usage.userViewGroupRate'), value: c.user_group_rate, format: 'rate' }
  ]
})

const calculationPanels = computed<CalculationPanel[]>(() => {
  if (!preview.value) return []
  return [
    buildCalculationPanel('real', t('admin.usage.userViewFormulaRealTitle'), preview.value.real),
    buildCalculationPanel('user', t('admin.usage.userViewFormulaUserTitle'), preview.value.user_view)
  ]
})

const buildCalculationPanel = (mode: FormulaMode, title: string, snapshot: UserViewSnapshot): CalculationPanel => {
  const rows = buildFormulaRows(snapshot, mode)
  const componentSubtotal = rows.reduce((sum, row) => sum + row.cost, 0)
  const otherCost = snapshot.total_cost - componentSubtotal
  const billedCost = snapshot.total_cost * snapshot.rate_multiplier
  return {
    key: mode,
    title,
    snapshot,
    rows,
    componentSubtotal,
    otherCost,
    hasOtherCost: Math.abs(otherCost) > epsilon,
    billedCost,
    diff: billedCost - snapshot.actual_cost
  }
}

const buildFormulaRows = (snapshot: UserViewSnapshot, mode: FormulaMode): FormulaRow[] => {
  const defs = [
    {
      key: 'input',
      label: t('usage.input'),
      tokens: snapshot.input_tokens,
      cost: snapshot.input_cost,
      price: mode === 'user' ? snapshot.display_input_price ?? null : null
    },
    {
      key: 'cache_read',
      label: t('usage.cacheRead'),
      tokens: snapshot.cache_read_tokens,
      cost: snapshot.cache_read_cost,
      price: mode === 'user' ? snapshot.display_cache_read_price ?? null : null
    },
    {
      key: 'cache_creation',
      label: t('usage.cacheCreation'),
      tokens: snapshot.cache_creation_tokens,
      cost: snapshot.cache_creation_cost,
      price: mode === 'user' ? snapshot.display_cache_creation_price ?? null : null
    },
    {
      key: 'output',
      label: t('usage.output'),
      tokens: snapshot.output_tokens,
      cost: snapshot.output_cost,
      price: mode === 'user' ? snapshot.display_output_price ?? null : null
    }
  ]

  return defs
    .filter(def => def.tokens > 0 || Math.abs(def.cost) > epsilon)
    .map(def => {
      const unitPrice = resolveUnitPrice(def.price, def.tokens, def.cost, mode === 'real')
      return {
        key: def.key,
        label: def.label,
        tokens: def.tokens,
        unitPrice,
        formula: formatFormula(def.tokens, unitPrice),
        cost: def.cost
      }
    })
}

const resolveUnitPrice = (
  configured: number | null,
  tokens: number,
  cost: number,
  allowCostFallback: boolean
): number | null => {
  if (configured != null && configured > 0) return configured
  if (allowCostFallback && tokens > 0 && cost > 0) return cost / tokens
  return null
}

const actualCostMismatch = computed(() => {
  if (!preview.value) return false
  return Math.abs(preview.value.real.actual_cost - preview.value.user_view.actual_cost) > 1e-9
})

const formatUsd = (v: number): string => `$${v.toFixed(6)}`

const formatSignedUsd = (v: number): string => {
  if (Math.abs(v) <= epsilon) return '$0.000000'
  const sign = v > 0 ? '+' : '-'
  return `${sign}$${Math.abs(v).toFixed(6)}`
}

const formatRate = (v: number): string => `x${v.toFixed(2)}`

const formatUnitPrice = (v: number | null): string => {
  if (v == null || v <= 0) return '-'
  return `$${(v * 1_000_000).toFixed(4)} / MTok`
}

const formatFormulaTokens = (v: number | null): string => {
  if (v == null) return '-'
  return v.toLocaleString()
}

const formatFormula = (tokens: number, unitPrice: number | null): string => {
  if (tokens <= 0 || unitPrice == null || unitPrice <= 0) return '-'
  return `${tokens.toLocaleString()} x ${formatUnitPrice(unitPrice)}`
}

const calcDiffClass = (diff: number): string => {
  if (Math.abs(diff) <= 1e-6) return 'text-emerald-600 dark:text-emerald-400'
  return 'text-amber-600 dark:text-amber-400'
}

const formatConfig = (v: number | null, f: ConfigFormat): string => {
  if (v == null) return '—'
  if (f === 'price') return `$${(v * 1_000_000).toFixed(2)} / MTok`
  if (f === 'ratio') return v.toFixed(4)
  return `×${v.toFixed(2)}`
}

const formatCell = (v: number, f: CellFormat): string => {
  if (f === 'int') return v.toLocaleString()
  if (f === 'usd') return `$${v.toFixed(6)}`
  return `×${v.toFixed(2)}`
}

const diffText = (real: number, user: number): string => {
  if (real === 0 && user === 0) return '—'
  if (real === 0) return user > 0 ? '+∞' : '−∞'
  const pct = (user - real) / Math.abs(real) * 100
  if (Math.abs(pct) < 0.01) return '±0%'
  const sign = pct > 0 ? '+' : ''
  return `${sign}${pct.toFixed(1)}%`
}

const diffClass = (real: number, user: number): string => {
  if (real === 0 && user === 0) return 'text-gray-400'
  if (real === 0) return 'text-emerald-600 dark:text-emerald-400 font-medium'
  const pct = (user - real) / Math.abs(real) * 100
  if (Math.abs(pct) < 0.01) return 'text-gray-500'
  return pct > 0
    ? 'text-emerald-600 dark:text-emerald-400 font-medium'
    : 'text-red-600 dark:text-red-400 font-medium'
}
</script>
