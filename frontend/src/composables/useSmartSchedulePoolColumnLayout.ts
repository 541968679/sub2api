import { computed, onMounted, onUnmounted, reactive, ref, watch, type ComputedRef } from 'vue'
import type { Column } from '@/components/common/types'
import {
  clampColumnWidth,
  isReorderableColumn,
  mergeAccountColumnOrder,
  moveAccountColumnOrder,
  parseAccountColumnLayout
} from '@/views/admin/accountColumnLayout'

export const SMART_SCHEDULE_POOL_HIDDEN_KEY = 'smart-schedule-pool-hidden-columns'
export const SMART_SCHEDULE_POOL_LAYOUT_KEY = 'smart-schedule-pool-column-layout'

const PINNED_VISIBLE = new Set(['select', 'name', 'actions'])

const DEFAULT_COLUMN_WIDTHS: Record<string, number> = {
  select: 100,
  name: 180,
  platform_type: 140,
  concurrency: 130,
  pair_cap: 120,
  admission: 140,
  status: 120,
  schedulable: 110,
  quality_ttft: 110,
  today_stats: 120,
  groups: 140,
  usage: 140,
  sort_order: 88,
  priority: 100,
  upstream_rate_multiplier: 110,
  last_used_at: 120,
  actions: 280
}

export function readSmartSchedulePoolFetchNeeds(): { quality: boolean; today: boolean } {
  const hidden = readHiddenColumns()
  return {
    quality: !hidden.has('quality_ttft'),
    today: !hidden.has('today_stats') || !hidden.has('usage')
  }
}

function readHiddenColumns(): Set<string> {
  const hidden = new Set<string>()
  if (typeof window === 'undefined') return hidden
  try {
    const saved = localStorage.getItem(SMART_SCHEDULE_POOL_HIDDEN_KEY)
    if (!saved) return hidden
    const parsed = JSON.parse(saved) as unknown
    if (!Array.isArray(parsed)) return hidden
    for (const key of parsed) {
      if (typeof key === 'string' && !PINNED_VISIBLE.has(key)) hidden.add(key)
    }
  } catch {
    return hidden
  }
  return hidden
}

export function useSmartSchedulePoolColumnLayout(allColumns: ComputedRef<Column[]>) {
  const showColumnDropdown = ref(false)
  const columnDropdownRef = ref<HTMLElement | null>(null)
  const hiddenColumns = reactive<Set<string>>(readHiddenColumns())
  const columnOrder = ref<string[]>([])
  const columnWidths = ref<Record<string, number>>({})

  const allKeys = computed(() => allColumns.value.map((col) => col.key))

  function getAllColumnKeys() {
    return allKeys.value
  }

  function saveHiddenColumns() {
    try {
      localStorage.setItem(SMART_SCHEDULE_POOL_HIDDEN_KEY, JSON.stringify([...hiddenColumns]))
    } catch {
      // ignore quota / private-mode failures
    }
  }

  function saveColumnLayout() {
    try {
      localStorage.setItem(
        SMART_SCHEDULE_POOL_LAYOUT_KEY,
        JSON.stringify({
          version: 1,
          order: columnOrder.value,
          widths: columnWidths.value
        })
      )
    } catch {
      // ignore quota / private-mode failures
    }
  }

  function loadSavedLayout() {
    const keys = getAllColumnKeys()
    if (keys.length === 0) return
    try {
      const layout = parseAccountColumnLayout(localStorage.getItem(SMART_SCHEDULE_POOL_LAYOUT_KEY), keys)
      columnOrder.value = layout.order
      columnWidths.value = layout.widths
    } catch {
      columnOrder.value = mergeAccountColumnOrder(null, keys)
      columnWidths.value = {}
    }
  }

  function ensureColumnOrderSynced() {
    const keys = getAllColumnKeys()
    const merged = mergeAccountColumnOrder(columnOrder.value.length ? columnOrder.value : null, keys)
    if (merged.join('|') !== columnOrder.value.join('|')) {
      columnOrder.value = merged
      saveColumnLayout()
    }
  }

  const orderedToggleableColumns = computed(() => {
    const byKey = new Map(allColumns.value.map((col) => [col.key, col]))
    const order = mergeAccountColumnOrder(
      columnOrder.value.length ? columnOrder.value : null,
      getAllColumnKeys()
    )
    return order
      .filter((key) => !PINNED_VISIBLE.has(key))
      .map((key) => byKey.get(key))
      .filter((col): col is Column => Boolean(col))
  })

  const visibleColumns = computed<Column[]>(() => {
    const byKey = new Map(allColumns.value.map((col) => [col.key, col]))
    const order = mergeAccountColumnOrder(
      columnOrder.value.length ? columnOrder.value : null,
      getAllColumnKeys()
    )
    return order
      .map((key) => byKey.get(key))
      .filter((col): col is Column => Boolean(col))
      .filter((col) => PINNED_VISIBLE.has(col.key) || !hiddenColumns.has(col.key))
      .map((col) => {
        const width = columnWidths.value[col.key] ?? DEFAULT_COLUMN_WIDTHS[col.key]
        return {
          ...col,
          width,
          minWidth: 64,
          resizable: col.key !== 'actions'
        }
      })
  })

  function isColumnVisible(key: string) {
    return PINNED_VISIBLE.has(key) || !hiddenColumns.has(key)
  }

  function toggleColumn(key: string) {
    if (PINNED_VISIBLE.has(key)) return
    if (hiddenColumns.has(key)) hiddenColumns.delete(key)
    else hiddenColumns.add(key)
    saveHiddenColumns()
  }

  function canMoveColumn(key: string, direction: 'up' | 'down') {
    if (!isReorderableColumn(key)) return false
    const order = mergeAccountColumnOrder(columnOrder.value, getAllColumnKeys())
    const index = order.indexOf(key)
    if (index < 0) return false
    const target = direction === 'up' ? index - 1 : index + 1
    if (target < 0 || target >= order.length) return false
    return isReorderableColumn(order[target])
  }

  function moveColumn(key: string, direction: 'up' | 'down') {
    ensureColumnOrderSynced()
    const next = moveAccountColumnOrder(columnOrder.value, key, direction)
    if (next.join('|') === columnOrder.value.join('|')) return
    columnOrder.value = next
    saveColumnLayout()
  }

  function handleColumnResize(key: string, width: number) {
    columnWidths.value = {
      ...columnWidths.value,
      [key]: clampColumnWidth(key, width)
    }
    saveColumnLayout()
  }

  function resetColumnLayout() {
    columnOrder.value = mergeAccountColumnOrder(null, getAllColumnKeys())
    columnWidths.value = {}
    saveColumnLayout()
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as Node | null
    if (columnDropdownRef.value && target && !columnDropdownRef.value.contains(target)) {
      showColumnDropdown.value = false
    }
  }

  watch(allKeys, () => {
    if (typeof window === 'undefined') return
    if (columnOrder.value.length === 0) loadSavedLayout()
    else ensureColumnOrderSynced()
  }, { immediate: true })

  onMounted(() => {
    document.addEventListener('click', handleClickOutside)
  })

  onUnmounted(() => {
    document.removeEventListener('click', handleClickOutside)
  })

  return {
    showColumnDropdown,
    columnDropdownRef,
    orderedToggleableColumns,
    visibleColumns,
    isColumnVisible,
    toggleColumn,
    canMoveColumn,
    moveColumn,
    handleColumnResize,
    resetColumnLayout
  }
}
