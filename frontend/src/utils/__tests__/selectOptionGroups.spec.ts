import { describe, expect, it } from 'vitest'
import {
  filterOptionsPreservingGroups,
  flattenPlatformSections,
  sectionsByPlatform,
  sectionsFromGroupedOptions
} from '../selectOptionGroups'

const label = (platform: string) =>
  ({ anthropic: 'Anthropic', openai: 'OpenAI', gemini: 'Gemini', zhipu: '智谱' })[platform] ?? platform

describe('sectionsByPlatform', () => {
  it('groups groups under a stable platform order and keeps input order inside a platform', () => {
    const sections = sectionsByPlatform(
      [
        { id: 1, name: 'OpenAI B', platform: 'openai' },
        { id: 2, name: 'Claude A', platform: 'anthropic' },
        { id: 3, name: 'OpenAI A', platform: 'openai' },
        { id: 4, name: '智谱线路', platform: 'zhipu' }
      ],
      label
    )

    expect(sections.map((section) => section.platform)).toEqual(['anthropic', 'openai', 'zhipu'])
    expect(sections[1].items.map((item) => item.name)).toEqual(['OpenAI B', 'OpenAI A'])
    expect(sections[2].label).toBe('智谱')
  })

  it('puts an unknown platform after the known ones', () => {
    const sections = sectionsByPlatform(
      [
        { id: 1, name: 'Future', platform: 'future-lab' },
        { id: 2, name: 'Gemini', platform: 'gemini' }
      ],
      label
    )

    expect(sections.map((section) => section.platform)).toEqual(['gemini', 'future-lab'])
  })
})

describe('filterOptionsPreservingGroups', () => {
  const options = flattenPlatformSections(
    sectionsByPlatform(
      [
        { value: 1, label: '优质', description: 'fast lane', platform: 'openai' },
        { value: 2, label: '经济', description: null, platform: 'openai' },
        { value: 3, label: 'Claude', description: 'sonnet', platform: 'anthropic' }
      ],
      label
    )
  )

  it('keeps the platform heading when only a group name matches', () => {
    const filtered = filterOptionsPreservingGroups(options, '优质')
    expect(filtered.map((option) => ('kind' in option && option.kind === 'group' ? option.label : option.label))).toEqual([
      'OpenAI',
      '优质'
    ])
  })

  it('shows every group in a platform when the platform name or id matches', () => {
    expect(filterOptionsPreservingGroups(options, '智谱')).toEqual([])
    const byName = filterOptionsPreservingGroups(options, 'OpenAI')
    const byId = filterOptionsPreservingGroups(options, 'zhipu')
    expect(byName.filter((option) => !('kind' in option && option.kind === 'group')).map((option) => option.label)).toEqual([
      '优质',
      '经济'
    ])
    expect(byId).toEqual([])
  })

  it('matches a platform id on the heading and keeps that section', () => {
    const withZhipu = flattenPlatformSections(
      sectionsByPlatform([{ value: 9, label: '线路', description: null, platform: 'zhipu' }], label)
    )
    const filtered = filterOptionsPreservingGroups(withZhipu, 'zhipu')
    expect(sectionsFromGroupedOptions(filtered)[0]).toMatchObject({
      platform: 'zhipu',
      label: '智谱'
    })
    expect(sectionsFromGroupedOptions(filtered)[0].items).toHaveLength(1)
  })

  it('filters a flat list by label and description', () => {
    const flat = [
      { value: 1, label: 'Alpha', description: 'one' },
      { value: 2, label: 'Beta', description: 'two' }
    ]
    expect(filterOptionsPreservingGroups(flat, 'two').map((option) => option.value)).toEqual([2])
    expect(filterOptionsPreservingGroups(flat, '   ')).toEqual(flat)
  })

  it('keeps ungrouped rows ahead of platform sections', () => {
    const rows = [{ value: '', label: '全部分组' }, ...options]
    const filtered = filterOptionsPreservingGroups(rows, 'Claude')
    expect(filtered.map((option) => option.label)).toEqual(['Anthropic', 'Claude'])
    expect(filterOptionsPreservingGroups(rows, '全部').map((option) => option.label)).toEqual(['全部分组'])
  })
})
