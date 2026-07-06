import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import ImageMonitorCard from '../ImageMonitorCard.vue'
import type { ImageMonitorPublicView } from '@/api/imageChannelMonitor'

const messages: Record<string, string> = {
  'channelStatus.imageSection.badge': '生图',
  'channelStatus.imageSection.apiLatency': '生图耗时',
  'channelStatus.imageSection.downloadLatency': '图片下载',
  'channelStatus.windowTab.7d': '近 7 天',
  'monitorCommon.availabilityPrefix': '可用率',
  'monitorCommon.status.operational': '正常',
  'monitorCommon.status.unknown': '未知',
  'monitorCommon.latencyEmpty': '-',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const raw = messages[key] ?? key
        if (!params) return raw
        return raw.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? ''))
      },
    }),
  }
})

function makeItem(overrides: Partial<ImageMonitorPublicView> = {}): ImageMonitorPublicView {
  return {
    id: 1,
    name: '生图通道A',
    model: 'gpt-image-1',
    latest_status: 'operational',
    latest_api_ms: 18234,
    latest_download_ms: 2100,
    availability_7d: 99.5,
    availability_15d: 98.2,
    availability_30d: 97.1,
    timeline: [{ status: 'operational', latency_ms: 18234, checked_at: new Date().toISOString() }],
    ...overrides,
  }
}

function mountCard(item = makeItem()) {
  return mount(ImageMonitorCard, {
    props: {
      item,
      window: '7d' as const,
      availabilityValue: item.availability_7d,
      countdownSeconds: 30,
    },
    global: {
      stubs: { MonitorTimeline: true },
    },
  })
}

describe('ImageMonitorCard', () => {
  it('renders public name, model, latency and availability', () => {
    const wrapper = mountCard()
    expect(wrapper.text()).toContain('生图通道A')
    expect(wrapper.text()).toContain('gpt-image-1')
    expect(wrapper.text()).toContain('18234')
    expect(wrapper.text()).toContain('2100')
    expect(wrapper.text()).toContain('99.50')
    expect(wrapper.text()).toContain('正常')
  })

  it('falls back to unknown status for channels without history', () => {
    const wrapper = mountCard(
      makeItem({ latest_status: 'empty', latest_api_ms: null, latest_download_ms: null, timeline: [] })
    )
    expect(wrapper.text()).toContain('未知')
  })

  it('emits click when card pressed', async () => {
    const wrapper = mountCard()
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('click')).toBeTruthy()
  })
})
