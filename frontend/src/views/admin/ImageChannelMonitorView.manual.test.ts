import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ImageChannelMonitorView from './ImageChannelMonitorView.vue'
import type {
  ImageChannelManualRunResponse,
  ImageChannelManualTestParams,
} from '@/api/admin/imageChannelMonitor'

const searchApiKeys = vi.fn()
const listMonitors = vi.fn()
const manualTest = vi.fn()
const getManualTestImage = vi.fn()
const getManualTestStatus = vi.fn()
const cancelManualTest = vi.fn()
const cancelManualTestByClientRunID = vi.fn()
const showError = vi.fn()

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    imageChannelMonitor: {
      list: (...args: unknown[]) => listMonitors(...args),
      getStatus: vi.fn().mockResolvedValue({ running: false }),
      manualTest: (...args: unknown[]) => manualTest(...args),
      getManualTestImage: (...args: unknown[]) => getManualTestImage(...args),
      getManualTestStatus: (...args: unknown[]) => getManualTestStatus(...args),
      cancelManualTest: (...args: unknown[]) => cancelManualTest(...args),
      cancelManualTestByClientRunID: (...args: unknown[]) =>
        cancelManualTestByClientRunID(...args),
    },
    usage: {
      searchApiKeys: (...args: unknown[]) => searchApiKeys(...args),
    },
  },
}))

function mountView() {
  return shallowMount(ImageChannelMonitorView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /></div>',
        },
        BaseDialog: true,
        ConfirmDialog: true,
        ImageMonitorStatusDialog: true,
        DataTable: true,
        Pagination: true,
        EmptyState: true,
        MonitorTimeline: true,
      },
    },
  })
}

async function openManualPanel(wrapper: VueWrapper) {
  const tabs = wrapper.findAll('[role="tab"]')
  await tabs[1].trigger('click')
  await flushPromises()
}

async function selectGatewayRequestContext(wrapper: VueWrapper) {
  await wrapper.get('[data-testid="manual-api-key"]').setValue('7')
  await wrapper.get('input[type="checkbox"][value="1"]').setValue(true)
}

function terminalRun(
  overrides: Partial<ImageChannelManualRunResponse> = {}
): ImageChannelManualRunResponse {
  return {
    run_id: 'run-1',
    monitor: undefined as unknown as ImageChannelManualRunResponse['monitor'],
    mode: 'generate',
    batch_id: 'batch-1',
    batch_size: 1,
    batch_index: 1,
    running: false,
    canceled: false,
    stage: 'completed',
    message: '',
    started_at: '2026-07-10T10:00:00Z',
    updated_at: '2026-07-10T10:00:01Z',
    completed_at: '2026-07-10T10:00:01Z',
    ...overrides,
  }
}

function installMemoryIndexedDB(options: { failedPuts?: number } = {}) {
  const records = new Map<string, Record<string, unknown>>()
  let failedPuts = options.failedPuts || 0
  const original = Object.getOwnPropertyDescriptor(window, 'indexedDB')
  const db = {
    objectStoreNames: { contains: () => true },
    createObjectStore: vi.fn(),
    close: vi.fn(),
    transaction: () => {
      const transaction: {
        error: Error | null
        oncomplete?: () => void
        onerror?: () => void
        objectStore: () => {
          put: (value: Record<string, unknown> & { ref: string }) => void
          get: (key: string) => Record<string, unknown>
          delete: (key: string) => void
        }
      } = {
        error: null,
        objectStore: () => ({
          put: (value) => {
            if (failedPuts > 0) {
              failedPuts -= 1
              transaction.error = new Error('IndexedDB quota exceeded')
              queueMicrotask(() => transaction.onerror?.())
              return
            }
            records.set(value.ref, value)
            queueMicrotask(() => transaction.oncomplete?.())
          },
          get: (key) => {
            const request: Record<string, unknown> = {}
            queueMicrotask(() => {
              request.result = records.get(key)
              ;(request.onsuccess as (() => void) | undefined)?.()
            })
            return request
          },
          delete: (key) => {
            records.delete(key)
            queueMicrotask(() => transaction.oncomplete?.())
          },
        }),
      }
      return transaction
    },
  }
  const indexedDB = {
    open: () => {
      const request: Record<string, unknown> = { result: db, error: null }
      queueMicrotask(() => (request.onsuccess as (() => void) | undefined)?.())
      return request
    },
  }
  Object.defineProperty(window, 'indexedDB', {
    configurable: true,
    value: indexedDB,
  })
  return {
    records,
    restore: () => {
      if (original) Object.defineProperty(window, 'indexedDB', original)
      else delete (window as Window & { indexedDB?: IDBFactory }).indexedDB
    },
  }
}

describe('ImageChannelMonitorView manual gateway mode', () => {
  beforeEach(() => {
    searchApiKeys.mockReset().mockResolvedValue([
      { id: 7, name: 'diagnostic-key', user_id: 11 },
    ])
    listMonitors.mockReset().mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'account-channel',
          source_type: 'account',
          account_id: 42,
          account_name: 'image-account',
          endpoint: '',
          model: 'gpt-image-1',
        },
      ],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    manualTest.mockReset()
    getManualTestImage.mockReset()
    getManualTestStatus.mockReset()
    cancelManualTest.mockReset()
    cancelManualTestByClientRunID.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    window.localStorage.clear()
  })

  it('defaults to account gateway mode and accepts an independent image pool', async () => {
    const wrapper = mountView()
    await openManualPanel(wrapper)

    expect(searchApiKeys).toHaveBeenCalledWith(undefined, '')
    expect(
      wrapper.get('[data-testid="manual-execution-mode-gateway-account"]').attributes('aria-selected')
    ).toBe('true')
    expect(wrapper.get('[data-testid="manual-api-key"]').exists()).toBe(true)
    const modeTabs = wrapper.findAll('button[role="tab"]')
    const editTab = modeTabs.find((tab) => tab.text().includes('admin.imageChannelMonitor.manual.edit'))
    expect(editTab).toBeDefined()
    await editTab!.trigger('click')
    expect(wrapper.get('[data-testid="manual-input-images"]').attributes()).toHaveProperty('multiple')
  })

  it('orchestrates c20 as independent gateway launches with distinct client run ids', async () => {
    manualTest.mockImplementation((_id: number, payload: ImageChannelManualTestParams) =>
      Promise.resolve(
        terminalRun({
          run_id: `run-c20-${payload.batch_index}`,
          batch_id: payload.batch_id,
          batch_size: payload.batch_size,
          batch_index: payload.batch_index,
          gateway_status: 'succeeded',
          delivery_status: 'not_requested',
          observation_status: 'observable',
          result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
        } as Partial<ImageChannelManualRunResponse>)
      )
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const concurrencyInput = wrapper
        .findAll('input[type="number"]')
        .find((input) => input.attributes('max') === '20')
      expect(concurrencyInput).toBeDefined()
      await concurrencyInput!.setValue(20)

      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')

      await vi.waitFor(() => expect(manualTest).toHaveBeenCalledTimes(20))
      const payloads = manualTest.mock.calls.map((call) => call[1] as ImageChannelManualTestParams)
      expect(new Set(payloads.map((payload) => payload.client_run_id)).size).toBe(20)
      expect(payloads.map((payload) => payload.batch_index)).toEqual(
        Array.from({ length: 20 }, (_, index) => index + 1)
      )
      expect(payloads.every((payload) => payload.batch_size === 20)).toBe(true)
    } finally {
      wrapper.unmount()
    }
  })

  it('maps an expired backend observation to observation lost instead of image failure', async () => {
    manualTest.mockResolvedValueOnce(
      terminalRun({
        observation_status: 'expired',
        gateway_status: 'pending',
        delivery_status: 'pending',
        stage: 'expired',
        message: 'manual run metadata expired',
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()
    await openManualPanel(wrapper)
    await selectGatewayRequestContext(wrapper)

    const startButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
    expect(startButton).toBeDefined()
    await startButton!.trigger('click')
    await flushPromises()

    const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
    expect(resultRow).toBeDefined()
    expect(resultRow!.text()).toContain('admin.imageChannelMonitor.manual.observationLost')
    expect(resultRow!.text()).not.toContain('admin.imageChannelMonitor.status.error')
  })

  it('keeps recovering an idempotent gateway launch beyond the initial control retry budget', async () => {
    manualTest
      .mockRejectedValueOnce({ status: 503, code: 'SERVICE_UNAVAILABLE' })
      .mockRejectedValueOnce({ status: 504, code: 'GATEWAY_TIMEOUT' })
      .mockRejectedValueOnce({ status: 0, code: 'ERR_NETWORK' })
      .mockRejectedValueOnce({ status: 500, code: 'INTERNAL_ERROR' })
      .mockRejectedValueOnce({ status: 408, code: 'REQUEST_TIMEOUT' })
      .mockResolvedValueOnce(
        terminalRun({
          gateway_status: 'succeeded',
          delivery_status: 'not_requested',
          observation_status: 'observable',
          result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
        } as Partial<ImageChannelManualRunResponse>)
      )
    const wrapper = mountView()
    await openManualPanel(wrapper)
    await selectGatewayRequestContext(wrapper)
    vi.useFakeTimers()

    try {
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(30_000)
      await flushPromises()

      expect(manualTest).toHaveBeenCalledTimes(6)
      const payloads = manualTest.mock.calls.map((call) => call[1] as Record<string, unknown>)
      expect(payloads.every((payload) => payload === payloads[0])).toBe(true)
      expect(new Set(payloads.map((payload) => payload.client_run_id))).toEqual(
        new Set([payloads[0].client_run_id])
      )
      expect(payloads[0].client_run_id).toEqual(expect.stringMatching(/^mcr-/))
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

  it('stops launch recovery when the user cancels before a run id is observed', async () => {
    vi.useFakeTimers()
    manualTest.mockRejectedValue({ status: 0, code: 'ERR_NETWORK' })
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const cancelButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.cancelAll'))
      expect(cancelButton).toBeDefined()
      await cancelButton!.trigger('click')
      await vi.advanceTimersByTimeAsync(30_000)
      await flushPromises()

      expect(manualTest).toHaveBeenCalledTimes(1)
      expect(cancelManualTest).not.toHaveBeenCalled()
      const clientRunID = (manualTest.mock.calls[0]?.[1] as { client_run_id: string }).client_run_id
      expect(cancelManualTestByClientRunID).toHaveBeenCalledWith(1, clientRunID)
      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow?.text()).toContain('admin.imageChannelMonitor.manual.canceled')
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('cancels the backend run when an in-flight launch response arrives after user cancellation', async () => {
    let resolveLaunch!: (run: ImageChannelManualRunResponse) => void
    manualTest.mockReturnValueOnce(
      new Promise<ImageChannelManualRunResponse>((resolve) => {
        resolveLaunch = resolve
      })
    )
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-next-batch',
        gateway_status: 'succeeded',
        delivery_status: 'not_requested',
        observation_status: 'observable',
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    cancelManualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-late-launch',
        canceled: true,
        stage: 'canceled',
        gateway_status: 'canceled',
        delivery_status: 'canceled',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const cancelButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.cancelAll'))
      expect(cancelButton).toBeDefined()
      await cancelButton!.trigger('click')

      let nextStartButton: ReturnType<VueWrapper['findAll']>[number] | undefined
      await vi.waitFor(() => {
        nextStartButton = wrapper
          .findAll('button')
          .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
        expect(nextStartButton).toBeDefined()
      })
      await nextStartButton!.trigger('click')
      await flushPromises()

      resolveLaunch(
        terminalRun({
          run_id: 'run-late-launch',
          running: true,
          completed_at: null,
          stage: 'gateway',
          gateway_status: 'pending',
          delivery_status: 'pending',
          observation_status: 'observable',
        } as Partial<ImageChannelManualRunResponse>)
      )
      await flushPromises()

      expect(cancelManualTest).toHaveBeenCalledWith(1, 'run-late-launch')
    } finally {
      wrapper.unmount()
    }
  })

  it('unlocks a new batch while an unreachable cancel endpoint retries in the background', async () => {
    vi.useFakeTimers()
    let resolveLaunch!: (run: ImageChannelManualRunResponse) => void
    manualTest.mockReturnValueOnce(
      new Promise<ImageChannelManualRunResponse>((resolve) => {
        resolveLaunch = resolve
      })
    )
    cancelManualTestByClientRunID.mockRejectedValue({ status: 0, code: 'ERR_NETWORK' })
    cancelManualTest.mockResolvedValue(
      terminalRun({
        run_id: 'run-late-cancel-retry',
        canceled: true,
        stage: 'canceled',
        gateway_status: 'canceled',
        delivery_status: 'canceled',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const cancelButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.cancelAll'))
      expect(cancelButton).toBeDefined()
      await cancelButton!.trigger('click')
      await flushPromises()

      const nextStartButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(nextStartButton).toBeDefined()
      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow?.text()).toContain('admin.imageChannelMonitor.manual.canceled')
      expect(showError).not.toHaveBeenCalled()

      resolveLaunch(
        terminalRun({
          run_id: 'run-late-cancel-retry',
          running: true,
          completed_at: null,
          stage: 'gateway',
          gateway_status: 'pending',
          delivery_status: 'pending',
          observation_status: 'observable',
        } as Partial<ImageChannelManualRunResponse>)
      )
      await flushPromises()

      expect(cancelManualTest).toHaveBeenCalledWith(1, 'run-late-cancel-retry')
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('ends the local batch while an observed backend cancel retries in the background', async () => {
    vi.useFakeTimers()
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-cancel-fails',
        running: true,
        completed_at: null,
        stage: 'gateway',
        gateway_status: 'pending',
        delivery_status: 'pending',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    getManualTestStatus.mockImplementation(() => new Promise(() => undefined))
    cancelManualTest.mockRejectedValue({ status: 503, code: 'SERVICE_UNAVAILABLE' })
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const cancelButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.cancelAll'))
      expect(cancelButton).toBeDefined()
      await cancelButton!.trigger('click')
      await flushPromises()

      expect(
        wrapper.findAll('button').some((button) =>
          button.text().includes('admin.imageChannelMonitor.manual.startWithCount')
        )
      ).toBe(true)
      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow?.text()).toContain('admin.imageChannelMonitor.manual.canceled')
      expect(showError).not.toHaveBeenCalled()
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('retries every transient cancel status for an observed run without showing a network error', async () => {
    vi.useFakeTimers()
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-cancel-retry',
        running: true,
        completed_at: null,
        stage: 'gateway',
        gateway_status: 'pending',
        delivery_status: 'pending',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    getManualTestStatus.mockImplementation(() => new Promise(() => undefined))
    for (const status of [0, 408, 425, 429, 500, 503]) {
      cancelManualTest.mockRejectedValueOnce({ status, code: `HTTP_${status}` })
    }
    cancelManualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-cancel-retry',
        canceled: true,
        stage: 'canceled',
        gateway_status: 'canceled',
        delivery_status: 'canceled',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const cancelButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.cancelAll'))
      expect(cancelButton).toBeDefined()
      await cancelButton!.trigger('click')
      await vi.advanceTimersByTimeAsync(20_000)
      await flushPromises()

      expect(cancelManualTest).toHaveBeenCalledTimes(7)
      expect(showError).not.toHaveBeenCalled()
      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow?.text()).toContain('admin.imageChannelMonitor.manual.canceled')
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('does not replay a non-idempotent direct probe launch after a network error', async () => {
    vi.useFakeTimers()
    manualTest.mockRejectedValueOnce({ status: 0, code: 'ERR_NETWORK' })
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await wrapper.get('[data-testid="manual-execution-mode-direct-probe"]').trigger('click')
      await wrapper.get('input[type="checkbox"][value="1"]').setValue(true)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(30_000)
      await flushPromises()

      expect(manualTest).toHaveBeenCalledTimes(1)
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('stores artifact bytes before persisting history metadata', async () => {
    const memoryDB = installMemoryIndexedDB()
    const artifactBlob = new Blob(['generated-image'], { type: 'image/png' })
    getManualTestImage
      .mockRejectedValueOnce({ status: 503, code: 'SERVICE_UNAVAILABLE' })
      .mockResolvedValueOnce(artifactBlob)
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-artifact',
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 15, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')

      await vi.waitFor(() => {
        const rawHistory = window.localStorage.getItem(
          'sub2api:image-channel-monitor:manual-history:v1'
        )
        expect(rawHistory).not.toBeNull()
        const history = JSON.parse(rawHistory || '[]') as Array<{
          run_id: string
          output_image_ref: string
        }>
        expect(history[0]?.run_id).toBe('run-artifact')
        expect(history[0]?.output_image_ref).toMatch(/^history-output:/)
        expect(memoryDB.records.has(history[0]?.output_image_ref)).toBe(true)
        const stored = memoryDB.records.get(history[0]?.output_image_ref)
        expect(stored?.blob).toBe(artifactBlob)
        expect(stored).not.toHaveProperty('data')
      })
      expect(getManualTestImage).toHaveBeenCalledTimes(2)
      expect(getManualTestImage).toHaveBeenNthCalledWith(
        1,
        1,
        'run-artifact',
        0,
        expect.objectContaining({ timeoutSeconds: 300 })
      )
      expect(getManualTestImage).toHaveBeenNthCalledWith(
        2,
        1,
        'run-artifact',
        0,
        expect.objectContaining({ timeoutSeconds: 300 })
      )
    } finally {
      wrapper.unmount()
      memoryDB.restore()
    }
  })

  it('treats an HTTP 200 empty artifact blob as transient and keeps recovering it', async () => {
    vi.useFakeTimers()
    const memoryDB = installMemoryIndexedDB()
    const artifactBlob = new Blob(['recovered-image'], { type: 'image/png' })
    getManualTestImage
      .mockResolvedValueOnce(new Blob([], { type: 'image/png' }))
      .mockResolvedValueOnce(artifactBlob)
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-empty-artifact',
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 15, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(300)
      await flushPromises()

      await vi.waitFor(() => {
        expect(getManualTestImage).toHaveBeenCalledTimes(2)
        const history = JSON.parse(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
        ) as Array<{ output_image_ref?: string; output_artifact_pending?: boolean }>
        expect(history[0]?.output_artifact_pending).toBe(false)
        expect(memoryDB.records.get(history[0]?.output_image_ref || '')?.blob).toBe(artifactBlob)
      })
    } finally {
      wrapper.unmount()
      memoryDB.restore()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('keeps a downloaded blob preview and artifact recovery pending when IndexedDB persistence fails', async () => {
    const memoryDB = installMemoryIndexedDB({ failedPuts: 1 })
    const createObjectURLDescriptor = Object.getOwnPropertyDescriptor(URL, 'createObjectURL')
    const revokeObjectURLDescriptor = Object.getOwnPropertyDescriptor(URL, 'revokeObjectURL')
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:downloaded-artifact'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
    const artifactBlob = new Blob(['downloaded-but-not-persisted'], { type: 'image/png' })
    getManualTestImage.mockResolvedValue(artifactBlob)
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-idb-recovery',
        started_at: new Date(Date.now() - 1_000).toISOString(),
        completed_at: new Date().toISOString(),
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 28, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')

      await vi.waitFor(() => {
        const history = JSON.parse(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
        ) as Array<{ output_artifact_pending?: boolean; output_image_ref?: string }>
        expect(history[0]?.output_artifact_pending).toBe(true)
        expect(history[0]?.output_image_ref).toBe('')
        expect(wrapper.find('img[src="blob:downloaded-artifact"]').exists()).toBe(true)
      })

      const retryButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('common.retry'))
      expect(retryButton).toBeDefined()
      await retryButton!.trigger('click')

      await vi.waitFor(() => {
        const history = JSON.parse(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
        ) as Array<{ output_artifact_pending?: boolean; output_image_ref?: string }>
        expect(history[0]?.output_artifact_pending).toBe(false)
        expect(memoryDB.records.get(history[0]?.output_image_ref || '')?.blob).toBe(artifactBlob)
      })
    } finally {
      wrapper.unmount()
      memoryDB.restore()
      if (createObjectURLDescriptor) {
        Object.defineProperty(URL, 'createObjectURL', createObjectURLDescriptor)
      } else {
        delete (URL as typeof URL & { createObjectURL?: typeof URL.createObjectURL }).createObjectURL
      }
      if (revokeObjectURLDescriptor) {
        Object.defineProperty(URL, 'revokeObjectURL', revokeObjectURLDescriptor)
      } else {
        delete (URL as typeof URL & { revokeObjectURL?: typeof URL.revokeObjectURL }).revokeObjectURL
      }
    }
  })

  it('recovers a generated artifact after the initial download retry budget is exhausted', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-10T10:00:02.000Z'))
    const memoryDB = installMemoryIndexedDB()
    const artifactBlob = new Blob(['eventually-downloaded'], { type: 'image/png' })
    getManualTestImage
      .mockRejectedValueOnce({ status: 502, code: 'BAD_GATEWAY' })
      .mockRejectedValueOnce({ status: 503, code: 'SERVICE_UNAVAILABLE' })
      .mockRejectedValueOnce({ status: 504, code: 'GATEWAY_TIMEOUT' })
      .mockResolvedValueOnce(artifactBlob)
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-artifact-recovery',
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 21, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(300)
      await vi.advanceTimersByTimeAsync(600)
      await flushPromises()

      expect(getManualTestImage).toHaveBeenCalledTimes(3)
      const pendingHistory = JSON.parse(
        window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
      ) as Array<{ output_artifact_pending?: boolean }>
      expect(pendingHistory[0]?.output_artifact_pending).toBe(true)

      await vi.advanceTimersByTimeAsync(1_500)
      await flushPromises()

      await vi.waitFor(() => {
        const history = JSON.parse(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
        ) as Array<{ output_image_ref?: string; output_artifact_pending?: boolean }>
        expect(history[0]?.output_artifact_pending).toBe(false)
        expect(history[0]?.output_image_ref).toMatch(/^history-output:/)
        expect(memoryDB.records.get(history[0]?.output_image_ref || '')?.blob).toBe(artifactBlob)
      })
    } finally {
      wrapper.unmount()
      memoryDB.restore()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('keeps recovering a generated artifact beyond four transient failures while it is retained', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-10T10:00:02.000Z'))
    const memoryDB = installMemoryIndexedDB()
    const artifactBlob = new Blob(['recovered-after-a-long-outage'], { type: 'image/png' })
    getManualTestImage
      .mockRejectedValueOnce({ status: 502, code: 'BAD_GATEWAY' })
      .mockRejectedValueOnce({ status: 503, code: 'SERVICE_UNAVAILABLE' })
      .mockRejectedValueOnce({ status: 504, code: 'GATEWAY_TIMEOUT' })
      .mockRejectedValueOnce({ status: 0, code: 'ERR_NETWORK' })
      .mockRejectedValueOnce({ status: 503, code: 'SERVICE_UNAVAILABLE' })
      .mockRejectedValueOnce({ status: 502, code: 'BAD_GATEWAY' })
      .mockRejectedValueOnce({ status: 504, code: 'GATEWAY_TIMEOUT' })
      .mockRejectedValueOnce({ status: 0, code: 'ERR_NETWORK' })
      .mockResolvedValueOnce(artifactBlob)
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-artifact-long-recovery',
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 29, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      await vi.advanceTimersByTimeAsync(60_000)
      await flushPromises()

      await vi.waitFor(() => {
        expect(getManualTestImage).toHaveBeenCalledTimes(9)
        const history = JSON.parse(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1') || '[]'
        ) as Array<{
          output_image_ref?: string
          output_artifact_pending?: boolean
          output_artifact_expires_at?: string
        }>
        expect(history[0]?.output_artifact_pending).toBe(false)
        expect(history[0]?.output_artifact_expires_at).toBe('2026-07-10T10:30:01.000Z')
        expect(memoryDB.records.get(history[0]?.output_image_ref || '')?.blob).toBe(artifactBlob)
      })
    } finally {
      wrapper.unmount()
      memoryDB.restore()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })

  it('does not retry a real artifact HTTP error', async () => {
    getManualTestImage.mockRejectedValueOnce({ status: 404, code: 'ARTIFACT_NOT_FOUND' })
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-artifact-404',
        gateway_status: 'succeeded',
        delivery_status: 'succeeded',
        observation_status: 'observable',
        artifacts: [{ index: 0, content_type: 'image/png', size: 15, source: 'b64_json' }],
        result: { status: 'operational' } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')

      await vi.waitFor(() => {
        expect(getManualTestImage).toHaveBeenCalledTimes(1)
        const rawHistory = window.localStorage.getItem(
          'sub2api:image-channel-monitor:manual-history:v1'
        )
        expect(rawHistory).toContain('run-artifact-404')
      })
      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow?.text()).toContain('admin.imageChannelMonitor.status.operational')
    } finally {
      wrapper.unmount()
    }
  })

  it('downloads the first available artifact when an earlier image failed delivery', async () => {
	const memoryDB = installMemoryIndexedDB()
	getManualTestImage.mockResolvedValueOnce(new Blob(['generated-image'], { type: 'image/png' }))
	manualTest.mockResolvedValueOnce(
	  terminalRun({
		run_id: 'run-partial-artifact',
		gateway_status: 'succeeded',
		delivery_status: 'succeeded',
		observation_status: 'observable',
		artifacts: [{ index: 1, content_type: 'image/png', size: 15, source: 'url' }],
		result: {
		  status: 'degraded',
		  error_stage: 'image_download',
		} as ImageChannelManualRunResponse['result'],
	  } as Partial<ImageChannelManualRunResponse>)
	)
	const wrapper = mountView()

	try {
	  await openManualPanel(wrapper)
	  await selectGatewayRequestContext(wrapper)
	  const startButton = wrapper
		.findAll('button')
		.find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
	  expect(startButton).toBeDefined()
	  await startButton!.trigger('click')

	  await vi.waitFor(() => {
		expect(getManualTestImage).toHaveBeenCalledWith(
		  1,
		  'run-partial-artifact',
		  1,
		  expect.objectContaining({ timeoutSeconds: 300 })
		)
		const rawHistory = window.localStorage.getItem(
		  'sub2api:image-channel-monitor:manual-history:v1'
		)
		expect(rawHistory).toContain('run-partial-artifact')
	  })
	} finally {
	  wrapper.unmount()
	  memoryDB.restore()
	}
  })

  it('does not request an artifact when the backend explicitly returns an empty artifact list', async () => {
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-no-artifact',
        gateway_status: 'succeeded',
        delivery_status: 'not_requested',
        observation_status: 'observable',
        artifacts: [],
        result: {
          status: 'operational',
          has_url: true,
          returned_image_url: 'https://images.example/result.png',
        } as ImageChannelManualRunResponse['result'],
      } as Partial<ImageChannelManualRunResponse>)
    )
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')

      await vi.waitFor(() => {
        expect(
          window.localStorage.getItem('sub2api:image-channel-monitor:manual-history:v1')
        ).toContain('run-no-artifact')
      })
      expect(getManualTestImage).not.toHaveBeenCalled()
    } finally {
      wrapper.unmount()
    }
  })

  it('maps an HTTP 410 status observation to observation lost immediately', async () => {
    vi.useFakeTimers()
    manualTest.mockResolvedValueOnce(
      terminalRun({
        run_id: 'run-observation-expired',
        running: true,
        completed_at: null,
        stage: 'running',
        gateway_status: 'pending',
        delivery_status: 'pending',
        observation_status: 'observable',
      } as Partial<ImageChannelManualRunResponse>)
    )
    getManualTestStatus.mockRejectedValueOnce({
      status: 410,
      code: 'MANUAL_RUN_EXPIRED',
    })
    const wrapper = mountView()

    try {
      await openManualPanel(wrapper)
      await selectGatewayRequestContext(wrapper)
      const startButton = wrapper
        .findAll('button')
        .find((button) => button.text().includes('admin.imageChannelMonitor.manual.startWithCount'))
      expect(startButton).toBeDefined()
      await startButton!.trigger('click')
      await flushPromises()

      const resultRow = wrapper.findAll('tbody tr').find((row) => row.text().includes('account-channel'))
      expect(resultRow).toBeDefined()
      expect(resultRow!.text()).toContain('admin.imageChannelMonitor.manual.observationLost')
      expect(getManualTestStatus).toHaveBeenCalledTimes(1)
    } finally {
      wrapper.unmount()
      vi.clearAllTimers()
      vi.useRealTimers()
    }
  })
})
