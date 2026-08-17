import { computed, defineComponent } from 'vue'
import { describe, expect, it, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import {
  SMART_SCHEDULE_POOL_HIDDEN_KEY,
  readSmartSchedulePoolFetchNeeds,
  useSmartSchedulePoolColumnLayout
} from '../useSmartSchedulePoolColumnLayout'

const columns = computed(() => [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'quality_ttft', label: 'Quality', sortable: false },
  { key: 'actions', label: 'Actions', sortable: false }
])

function mountLayout() {
  let api!: ReturnType<typeof useSmartSchedulePoolColumnLayout>
  mount(
    defineComponent({
      setup() {
        api = useSmartSchedulePoolColumnLayout(columns)
        return () => null
      }
    })
  )
  return api
}

describe('useSmartSchedulePoolColumnLayout', () => {
  beforeEach(() => {
    localStorage.removeItem(SMART_SCHEDULE_POOL_HIDDEN_KEY)
    localStorage.removeItem('smart-schedule-pool-column-layout')
  })

  it('keeps name and actions visible and persists hidden columns', () => {
    const layout = mountLayout()
    expect(layout.visibleColumns.value.map((col) => col.key)).toEqual(['name', 'quality_ttft', 'actions'])
    layout.toggleColumn('quality_ttft')
    expect(layout.isColumnVisible('quality_ttft')).toBe(false)
    expect(layout.visibleColumns.value.map((col) => col.key)).toEqual(['name', 'actions'])
    expect(JSON.parse(localStorage.getItem(SMART_SCHEDULE_POOL_HIDDEN_KEY) || '[]')).toContain('quality_ttft')
    layout.toggleColumn('name')
    expect(layout.isColumnVisible('name')).toBe(true)
  })

  it('remaps leftover usage hidden key to schedule_pnl', () => {
    localStorage.setItem(SMART_SCHEDULE_POOL_HIDDEN_KEY, JSON.stringify(['usage']))
    const needs = readSmartSchedulePoolFetchNeeds()
    expect(needs.pnl).toBe(false)
    expect(needs.today).toBe(true)
  })

  it('still fetches today stats when only the schedule pnl column is visible', () => {
    localStorage.setItem(SMART_SCHEDULE_POOL_HIDDEN_KEY, JSON.stringify(['today_stats']))
    expect(readSmartSchedulePoolFetchNeeds()).toEqual({ quality: true, today: true, pnl: true })
    localStorage.setItem(SMART_SCHEDULE_POOL_HIDDEN_KEY, JSON.stringify(['today_stats', 'schedule_pnl']))
    expect(readSmartSchedulePoolFetchNeeds()).toEqual({ quality: true, today: false, pnl: false })
  })
})
