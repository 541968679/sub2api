import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartScheduleAddAccountDialog from '../SmartScheduleAddAccountDialog.vue'
import type { Account } from '@/types'

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    props: ['show', 'title'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>'
  }
}))
vi.mock('@/components/common/Select.vue', () => ({
  default: {
    props: ['modelValue', 'options'],
    emits: ['update:modelValue'],
    template: `
      <select :value="modelValue" @change="$emit('update:modelValue', ($event.target).value)">
        <option v-for="opt in options" :key="String(opt.value)" :value="opt.value">{{ opt.label }}</option>
      </select>
    `
  }
}))
vi.mock('@/components/common/PlatformTypeBadge.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/account/AccountGroupsCell.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/admin/account/AccountTableFilters.vue', () => ({
  default: {
    props: ['searchQuery', 'filters', 'hidePlatform'],
    emits: ['update:searchQuery', 'update:filters'],
    template: `
      <div data-testid="account-table-filters">
        <input
          data-testid="add-dialog-search"
          :value="searchQuery"
          @input="$emit('update:searchQuery', ($event.target).value)"
        />
        <button data-testid="add-dialog-type-oauth" @click="$emit('update:filters', { ...filters, type: 'oauth' })" />
        <slot name="extra" />
      </div>
    `
  }
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (params) return key.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
        return key
      }
    })
  }
})

function account(overrides: Partial<Account> = {}): Account {
  return {
    id: 1,
    name: 'api-live',
    platform: 'anthropic',
    type: 'apikey',
    status: 'active',
    schedulable: true,
    proxy_id: null,
    concurrency: 1,
    priority: 0,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '',
    updated_at: '',
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  } as Account
}

function mountDialog(accounts: Account[]) {
  return mount(SmartScheduleAddAccountDialog, {
    props: {
      show: true,
      platform: 'anthropic',
      platformLabel: 'Anthropic',
      accounts,
      groups: [],
      proxies: [{ id: 8, name: 'proxy-8' } as never]
    }
  })
}

describe('SmartScheduleAddAccountDialog', () => {
  it('previews candidates and adds only the selected rows', async () => {
    const w = mountDialog([
      account({ id: 1, name: 'api-live', type: 'apikey' }),
      account({ id: 2, name: 'oauth-live', type: 'oauth' })
    ])
    expect(w.get('[data-testid="smart-schedule-add-dialog-preview"]').text()).toContain('api-live')
    expect(w.get('[data-testid="smart-schedule-add-dialog-preview"]').text()).toContain('oauth-live')
    await w.get('[data-testid="smart-schedule-add-dialog-check-2"]').setValue(true)
    await w.get('[data-testid="smart-schedule-add-dialog-selected"]').trigger('click')
    expect(w.emitted('add')?.[0]).toEqual([[2]])
  })

  it('filters the preview with account-list type and extra scheduling/proxy controls', async () => {
    const w = mountDialog([
      account({ id: 1, name: 'api-live', type: 'apikey' }),
      account({ id: 2, name: 'oauth-live', type: 'oauth' }),
      account({ id: 3, name: 'oauth-paused', type: 'oauth', schedulable: false, status: 'inactive' })
    ])
    await w.get('[data-testid="add-dialog-type-oauth"]').trigger('click')
    expect(w.get('[data-testid="smart-schedule-add-dialog-row-2"]').exists()).toBe(true)
    expect(w.get('[data-testid="smart-schedule-add-dialog-row-3"]').exists()).toBe(true)
    expect(w.find('[data-testid="smart-schedule-add-dialog-row-1"]').exists()).toBe(false)
    await w.get('[data-testid="smart-schedule-add-filter-scheduling"]').setValue('on')
    expect(w.find('[data-testid="smart-schedule-add-dialog-row-3"]').exists()).toBe(false)
    expect(w.get('[data-testid="smart-schedule-add-dialog-row-2"]').exists()).toBe(true)
    await w.get('[data-testid="smart-schedule-add-dialog-all"]').trigger('click')
    expect(w.emitted('add')?.[0]).toEqual([[2]])
  })

  it('applies a currently-scheduling API preset before add-all', async () => {
    const w = mountDialog([
      account({ id: 1, name: 'api-live', type: 'apikey' }),
      account({ id: 2, name: 'oauth-live', type: 'oauth' })
    ])
    await w.get('[data-testid="smart-schedule-add-dialog-preset-api"]').trigger('click')
    expect(w.find('[data-testid="smart-schedule-add-dialog-row-2"]').exists()).toBe(false)
    await w.get('[data-testid="smart-schedule-add-dialog-all"]').trigger('click')
    expect(w.emitted('add')?.[0]).toEqual([[1]])
  })
})
