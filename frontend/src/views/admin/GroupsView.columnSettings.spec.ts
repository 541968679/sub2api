import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, 'GroupsView.vue'), 'utf8')

describe('GroupsView column settings and used quota', () => {
  it('keeps name/actions fixed and validates persisted hidden columns', () => {
    expect(source).toContain('const ALWAYS_VISIBLE_COLUMNS = new Set(["name", "actions"])')
    expect(source).toContain('const HIDDEN_COLUMNS_KEY = "group-hidden-columns"')
    expect(source).toContain('validHiddenColumnKeys().has(key)')
    expect(source).toContain('@click="toggleColumn(col.key)"')
  })

  it('shows the existing total-cost summary as an independent used-quota column', () => {
    expect(source).toContain('{ key: "used_quota", label: t("admin.groups.columns.usedQuota"), sortable: false }')
    expect(source).toContain('<template #cell-used_quota="{ row }">')
    expect(source).toContain('usageMap.get(row.id)?.total_cost ?? 0')
  })

  it('loads summaries only while a consuming column is visible', () => {
    expect(source).toContain('isColumnVisible("usage") || isColumnVisible("used_quota")')
    expect(source).toContain('if (hasVisibleUsageSummaryConsumer.value) loadUsageSummary()')
    expect(source).toContain('if (hasVisibleCapacityColumn.value) loadCapacitySummary()')
  })
})
