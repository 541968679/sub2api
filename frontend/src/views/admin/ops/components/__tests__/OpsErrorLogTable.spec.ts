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
          ElTooltip: TooltipStub
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
          ElTooltip: TooltipStub
        }
      }
    })

    expect(wrapper.text()).toContain('admin.ops.errorLog.typeRecovered')
    expect(wrapper.text()).toContain('200')
  })
})
