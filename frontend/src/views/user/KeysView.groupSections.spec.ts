import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'KeysView.vue'), 'utf8')

describe('KeysView group picker sections', () => {
  it('sections the create-key select and the change-group menu by platform', () => {
    expect(source).toContain(':options="groupPickerSelectOptions"')
    expect(source).toContain('v-for="section in filteredGroupSections"')
    expect(source).toContain('isSelectGroupHeader(option)')
    expect(source).toContain('sectionsByPlatform')
  })
})
