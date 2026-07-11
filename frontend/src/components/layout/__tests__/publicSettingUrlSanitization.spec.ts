import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sources = {
  sidebar: readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8'),
  header: readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8'),
  home: readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8'),
  login: readFileSync(resolve(dir, '../../../views/auth/LoginView.vue'), 'utf8')
}

describe('public setting URL sanitization', () => {
  it('sanitizes every current site logo consumer', () => {
    expect(sources.sidebar).toContain('sanitizeUrl(appStore.siteLogo')
    expect(sources.home).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo')
    for (const source of [sources.sidebar, sources.home]) {
      expect(source).toContain('allowRelative: true')
      expect(source).toContain('allowDataUrl: true')
    }
  })

  it('sanitizes every current documentation link consumer', () => {
    expect(sources.header).toContain('sanitizeUrl(appStore.docUrl')
    expect(sources.home).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url')
    expect(sources.login).toContain('sanitizeUrl(appStore.docUrl')
  })
})
