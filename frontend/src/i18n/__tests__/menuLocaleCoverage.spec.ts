import { readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, extname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

const testDir = dirname(fileURLToPath(import.meta.url))
const srcRoot = resolve(testDir, '../..')
const routerPath = resolve(srcRoot, 'router/index.ts')
const sidebarPath = resolve(srcRoot, 'components/layout/AppSidebar.vue')

const staticKeyPatterns = [
  /\b(?:titleKey|descriptionKey)\s*:\s*['"`]([^'"`]+)['"`]/g,
  /\b(?:t|\$t)\(\s*['"`]([^'"`]+)['"`]/g,
  /\bi18n\.global\.t\(\s*['"`]([^'"`]+)['"`]/g,
]

const ignoredDirectories = new Set([
  'node_modules',
  'dist',
  'coverage',
  '__tests__',
])

const ignoredFiles = new Set([
  relative(srcRoot, resolve(srcRoot, 'i18n/locales/zh.ts')).replaceAll('\\', '/'),
  relative(srcRoot, resolve(srcRoot, 'i18n/locales/en.ts')).replaceAll('\\', '/'),
])

function walkSourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    const rel = relative(srcRoot, fullPath).replaceAll('\\', '/')
    const stat = statSync(fullPath)
    if (stat.isDirectory()) {
      if (!ignoredDirectories.has(entry)) {
        out.push(...walkSourceFiles(fullPath))
      }
      continue
    }
    if (ignoredFiles.has(rel)) continue
    if (['.ts', '.vue'].includes(extname(entry))) {
      out.push(fullPath)
    }
  }
  return out
}

function collectStaticI18nKeys(paths: string[]): Map<string, Set<string>> {
  const keysByFile = new Map<string, Set<string>>()
  for (const path of paths) {
    const source = readFileSync(path, 'utf8')
    const keys = new Set<string>()
    for (const pattern of staticKeyPatterns) {
      pattern.lastIndex = 0
      let match: RegExpExecArray | null
      while ((match = pattern.exec(source)) !== null) {
        const key = match[1]?.trim()
        if (key && !key.includes('${') && !key.endsWith('.')) {
          keys.add(key)
        }
      }
    }
    if (keys.size > 0) {
      keysByFile.set(relative(srcRoot, path).replaceAll('\\', '/'), keys)
    }
  }
  return keysByFile
}

function getMessage(messages: LocaleMessages, key: string): unknown {
  return key.split('.').reduce<unknown>((cursor, part) => {
    if (cursor && typeof cursor === 'object' && part in cursor) {
      return (cursor as Record<string, unknown>)[part]
    }
    return undefined
  }, messages)
}

function flattenKeyMap(keysByFile: Map<string, Set<string>>): string[] {
  const out: string[] = []
  for (const [file, keys] of keysByFile) {
    for (const key of keys) {
      out.push(`${file}: ${key}`)
    }
  }
  return out.sort()
}

function missingLocaleEntries(locale: LocaleMessages, keysByFile: Map<string, Set<string>>): string[] {
  const missing: string[] = []
  for (const [file, keys] of keysByFile) {
    for (const key of keys) {
      const value = getMessage(locale, key)
      if (value === undefined || value === key || value === '') {
        missing.push(`${file}: ${key}`)
      }
    }
  }
  return missing.sort()
}

describe('i18n menu and static key coverage', () => {
  it('keeps route and sidebar keys translated in zh and en', () => {
    const keysByFile = collectStaticI18nKeys([routerPath, sidebarPath])
    expect(flattenKeyMap(keysByFile)).not.toEqual([])
    expect(missingLocaleEntries(zh, keysByFile)).toEqual([])
    expect(missingLocaleEntries(en, keysByFile)).toEqual([])
  })

  it('keeps all static t() keys translated in zh and en', () => {
    const keysByFile = collectStaticI18nKeys(walkSourceFiles(srcRoot))
    expect(flattenKeyMap(keysByFile)).not.toEqual([])
    expect(missingLocaleEntries(zh, keysByFile)).toEqual([])
    expect(missingLocaleEntries(en, keysByFile)).toEqual([])
  })
})
