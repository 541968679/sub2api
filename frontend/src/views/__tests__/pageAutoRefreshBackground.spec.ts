import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewsDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')

function readView(rel: string): string {
  return readFileSync(resolve(viewsDir, rel), 'utf8')
}

describe('page auto-refresh keeps running in background tabs', () => {
  it('does not skip account or user list ticks when the tab is hidden', () => {
    expect(readView('admin/AccountsView.vue')).not.toContain('if (document.hidden) return')
    expect(readView('admin/UsersView.vue')).not.toContain('if (document.hidden) return')
  })

  it('pauses channel-status refresh only while a reload is in flight', () => {
    const source = readView('user/ChannelStatusView.vue')
    expect(source).toContain('shouldPause: () => loading.value')
    expect(source).not.toContain('document.hidden')
  })

  it('keeps backup and restore polling while the tab is hidden', () => {
    const source = readView('admin/BackupView.vue')
    expect(source).not.toContain('handleVisibilityChange')
    expect(source).not.toContain('visibilitychange')
  })

  it('refreshes dashboard stats on a mounted interval even when the tab is hidden', () => {
    const source = readView('user/DashboardView.vue')
    expect(source).toContain('statsTimer = setInterval(refreshStatsSilently, 60000)')
    expect(source).not.toContain(
      "setInterval(() => { if (document.visibilityState === 'visible') refreshStatsSilently() }"
    )
  })
})
