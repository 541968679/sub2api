import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'

const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      importData: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')
  })

  it('无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
        }
      }
    })

    const input = wrapper.find('input[type="file"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailed')
  })

  it('merges multiple selected JSON files before importing', async () => {
    const { adminAPI } = await import('@/api/admin')
    vi.mocked(adminAPI.accounts.importData).mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 2,
      account_failed: 0
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } }
    })

    const input = wrapper.find('input[type="file"]')
    const firstPayload = { type: 'sub2api', version: 1, proxies: [], accounts: [{ name: 'a' }], skipped_shadows: 1 }
    const secondPayload = { proxies: [{ proxy_key: 'p' }], accounts: [{ name: 'b' }], skipped_shadows: 2 }
    const first = new File([JSON.stringify(firstPayload)], 'first.json', { type: 'application/json' })
    const second = new File([JSON.stringify(secondPayload)], 'second.json', { type: 'application/json' })
    Object.defineProperty(first, 'text', { value: () => Promise.resolve(JSON.stringify(firstPayload)) })
    Object.defineProperty(second, 'text', { value: () => Promise.resolve(JSON.stringify(secondPayload)) })
    Object.defineProperty(input.element, 'files', { value: [first, second] })

    await input.trigger('change')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(adminAPI.accounts.importData).toHaveBeenCalledWith({
      data: expect.objectContaining({
        type: 'sub2api',
        version: 1,
        proxies: [{ proxy_key: 'p' }],
        accounts: [{ name: 'a' }, { name: 'b' }],
        skipped_shadows: 3
      }),
      skip_default_group_bind: true,
      auto_assign_proxy: undefined
    })
  })

  it('accepts dropped JSON files and rejects non-JSON drops', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } }
    })
    const zones = wrapper.findAll('div').filter((node) => node.attributes('class')?.includes('border-dashed'))
    expect(zones).toHaveLength(1)

    await zones[0].trigger('drop', { dataTransfer: { files: [new File(['x'], 'bad.txt', { type: 'text/plain' })] } })
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')

    const valid = new File(['{}'], 'valid.json', { type: 'application/json' })
    await zones[0].trigger('drop', { dataTransfer: { files: [valid] } })
    expect(wrapper.find('input[type="file"]').attributes('multiple')).toBeDefined()
    expect(wrapper.text()).toContain('valid.json')
  })
})
