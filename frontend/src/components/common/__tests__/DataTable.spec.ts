import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import DataTable from '../DataTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

let desktopViewportMatches = true
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: desktopViewportMatches,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false
  })
})

const columns = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'upstream_rate_multiplier', label: 'Upstream Rate', sortable: true }
]

describe('DataTable column sort affordance', () => {
  it('keeps sort indicators when a custom header slot is provided', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns,
        data: [{ id: 1, name: 'a', upstream_rate_multiplier: 0.15 }],
        serverSideSort: true,
        virtualScroll: false,
        resizableColumns: false,
        defaultSortKey: 'name'
      },
      slots: {
        'header-upstream_rate_multiplier': '<span>上游倍率</span>'
      }
    })
    await flushPromises()
    await nextTick()

    const indicators = wrapper.findAll('[data-testid="table-sort-indicator"]')
    expect(indicators.map((el) => el.attributes('data-sort-key'))).toEqual([
      'name',
      'upstream_rate_multiplier'
    ])

    const headers = wrapper.findAll('th')
    const upstreamHeader = headers.find((th) => th.text().includes('上游倍率'))
    expect(upstreamHeader).toBeTruthy()
    await upstreamHeader!.trigger('click')
    expect(wrapper.emitted('sort')?.[0]).toEqual(['upstream_rate_multiplier', 'asc'])
    expect(
      wrapper
        .find('[data-testid="table-sort-indicator"][data-sort-key="upstream_rate_multiplier"]')
        .attributes('data-sort-active')
    ).toBe('true')

    await upstreamHeader!.trigger('click')
    expect(wrapper.emitted('sort')?.[1]).toEqual(['upstream_rate_multiplier', 'desc'])
    wrapper.unmount()
  })

  it('keeps a hideOnMobile column in the desktop table and drops it from phone cards', async () => {
    const columnsWithHidden = [
      ...columns,
      { key: 'burn_rate', label: 'Burn', hideOnMobile: true },
      { key: 'actions', label: 'Actions' }
    ]
    const desktop = mount(DataTable, {
      props: {
        columns: columnsWithHidden,
        data: [{ id: 1, name: 'a', upstream_rate_multiplier: 0.15, burn_rate: 1 }],
        virtualScroll: false
      },
      slots: {
        'cell-burn_rate': '<span data-testid="desktop-burn">1.00</span>'
      }
    })
    await flushPromises()
    expect(desktop.findAll('th').map((th) => th.text())).toContain('Burn')
    expect(desktop.get('[data-testid="desktop-burn"]').exists()).toBe(true)
    desktop.unmount()

    desktopViewportMatches = false
    const phone = mount(DataTable, {
      props: {
        columns: columnsWithHidden,
        data: [{ id: 1, name: 'a', upstream_rate_multiplier: 0.15, burn_rate: 1 }],
        virtualScroll: false
      },
      slots: {
        'cell-burn_rate': '<span data-testid="phone-burn">1.00</span>',
        'cell-name': '<span data-testid="phone-name">a</span>'
      }
    })
    await flushPromises()
    expect(phone.find('[data-testid="phone-burn"]').exists()).toBe(false)
    expect(phone.get('[data-testid="phone-name"]').exists()).toBe(true)
    expect(phone.find('th').exists()).toBe(false)
    phone.unmount()
    desktopViewportMatches = true
  })
})
