import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { OpsErrorLog } from '@/api/admin/ops'
import OpsErrorLogTable from '../OpsErrorLogTable.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const TooltipStub = defineComponent({
  template: '<div><slot /></div>'
})

const row: OpsErrorLog = {
  id: 1,
  created_at: '2026-07-25T08:00:00Z',
  phase: 'upstream',
  type: 'invalid_request_error',
  error_owner: 'provider',
  error_source: 'upstream_http',
  severity: 'warning',
  status_code: 400,
  platform: 'openai',
  model: 'gpt-5.4',
  resolved: false,
  client_request_id: '',
  request_id: 'req_test',
  message: 'Unsupported content type',
  user_email: '',
  account_name: 'test account',
  group_name: 'OpenAI',
  inbound_endpoint: '/v1/responses',
  upstream_endpoint: '/v1/chat/completions'
}

describe('OpsErrorLogTable', () => {
  it('shows downstream to upstream endpoint mapping when they differ', () => {
    const wrapper = mount(OpsErrorLogTable, {
      props: {
        rows: [row],
        total: 1,
        loading: false,
        page: 1,
        pageSize: 20
      },
      global: {
        stubs: {
          Pagination: true,
          ElTooltip: TooltipStub,
          AddScheduleErrorWhitelistDialog: true
        }
      }
    })

    expect(wrapper.text()).toContain('/v1/responses -> /v1/chat/completions')
  })

  it('labels Recovered upstream-rescue rows', () => {
    const recovered: OpsErrorLog = {
      ...row,
      id: 2,
      status_code: 200,
      phase: 'upstream',
      error_owner: 'provider',
      message: 'Recovered upstream error 429: too many requests'
    }
    const wrapper = mount(OpsErrorLogTable, {
      props: {
        rows: [recovered],
        total: 1,
        loading: false,
        page: 1,
        pageSize: 20
      },
      global: {
        stubs: {
          Pagination: true,
          ElTooltip: TooltipStub,
          AddScheduleErrorWhitelistDialog: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.ops.errorLog.typeRecovered')
    expect(wrapper.text()).toContain('200')
  })

  it('marks Recovered as user-excluded and schedule-excluded when the toggle is off', () => {
    const recovered: OpsErrorLog = {
      ...row,
      id: 2,
      status_code: 200,
      phase: 'upstream',
      message: 'Recovered upstream error 429: too many requests',
      is_recovered: true,
      counted_in_user_error_rate: false,
      counted_in_account_compare_rate: true,
      counted_in_account_schedule_rate: false
    }
    const wrapper = mount(OpsErrorLogTable, {
      props: { rows: [recovered], total: 1, loading: false, page: 1, pageSize: 20 },
      global: { stubs: { Pagination: true, ElTooltip: TooltipStub, AddScheduleErrorWhitelistDialog: true } }
    })
    expect(wrapper.text()).toContain('admin.ops.errorLog.typeRecovered')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberUserExcluded')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberCompareIncluded')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberScheduleExcluded')
  })

  it('marks model-not-found as user-included and excluded from both account calibers', () => {
    const miss: OpsErrorLog = {
      ...row,
      id: 3,
      status_code: 404,
      phase: 'internal',
      type: 'api_error',
      error_owner: 'platform',
      message: 'model_not_found: claude-bad',
      counted_in_user_error_rate: true,
      counted_in_account_compare_rate: false,
      counted_in_account_schedule_rate: false,
      needs_ops_attention: true
    }
    const wrapper = mount(OpsErrorLogTable, {
      props: { rows: [miss], total: 1, loading: false, page: 1, pageSize: 20 },
      global: { stubs: { Pagination: true, ElTooltip: TooltipStub, AddScheduleErrorWhitelistDialog: true } }
    })
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberUserIncluded')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberCompareExcluded')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberScheduleExcluded')
    expect(wrapper.text()).toContain('admin.ops.errorLog.caliberNeedsOpsAttention')
  })

  it('shows upstream original as primary and mapped sentence as secondary without JSON', () => {
    const mapped: OpsErrorLog = {
      ...row,
      id: 4,
      status_code: 502,
      message: 'Upstream service temporarily unavailable',
      upstream_error_message: 'no enabled keys (any suffix) for model gpt-5.4',
      provider_error_code: 'channel:no_available_key'
    }
    const wrapper = mount(OpsErrorLogTable, {
      props: { rows: [mapped], total: 1, loading: false, page: 1, pageSize: 20 },
      global: { stubs: { Pagination: true, ElTooltip: TooltipStub, AddScheduleErrorWhitelistDialog: true } }
    })
    expect(wrapper.text()).toContain('channel:no_available_key')
    expect(wrapper.text()).toContain('no enabled keys')
    expect(wrapper.text()).toContain('Upstream service temporarily unavailable')
    expect(wrapper.text()).toContain('admin.ops.errorDetail.downstreamMapped')
    expect(wrapper.text()).not.toContain('"type":"new_api_error"')
    expect(wrapper.html()).not.toContain('error_body')
  })

  it('offers add-to-whitelist on each row', () => {
    const wrapper = mount(OpsErrorLogTable, {
      props: { rows: [row], total: 1, loading: false, page: 1, pageSize: 20 },
      global: { stubs: { Pagination: true, ElTooltip: TooltipStub, AddScheduleErrorWhitelistDialog: true } }
    })
    expect(wrapper.get('[data-test="schedule-error-whitelist-from-log-open"]').text()).toContain(
      'admin.ops.scheduleErrorWhitelist.addFromLog'
    )
  })
})
