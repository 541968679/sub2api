import type { BillingBasisHint, ModelPricingItem } from '@/api/admin/modelPricing'

export type MappingBillingObject = 'requested' | 'mapped'

/**
 * 模型配置表的"请求名/上游名"行。
 *
 * 行只有两种来源：
 * - 真实映射行：模型自身是某平台默认映射的键（含同名映射），一行一个平台条目，
 *   可编辑 / 删除 / 改计费对象。
 * - 直通行：模型在当前视图的任何平台里都没有自己的映射条目，展示 请求名 = 上游名，
 *   通过"添加映射"可以就地创建条目。
 *
 * 映射行只由映射键自己的条目产生（后端为每个映射键补 stub，保证条目存在），
 * 不再从映射目标条目反向展开行——旧实现的反向展开加去重会让同一请求名的行
 * 在不同条目间互相覆盖，映射目标看起来被改回请求名。
 */
export interface ModelNameRow {
  /** 行唯一 key：真实映射行为 `${platform}:${请求名}`，直通行为 `passthrough:${模型名}` */
  rowKey: string
  /** 请求模型名（映射键或模型自身） */
  mappingFrom: string
  /** 上游模型名 */
  upstreamDisplay: string
  /** 行所属平台（真实映射行必有；直通行取被映射来源的平台或空） */
  platform: string
  /** 是否为平台默认映射里的真实条目 */
  isMappingEntry: boolean
  billingObject: MappingBillingObject
  /** 映射到此模型的其他请求名（在直通/同名行上做提示） */
  mappedFrom: string[]
}

export function normalizeMappingBillingObject(value?: string | null): MappingBillingObject {
  return value === 'mapped' ? 'mapped' : 'requested'
}

function billingObjectForKey(hint: BillingBasisHint, key: string): MappingBillingObject {
  const perKey = hint.mapping_billing_objects?.[key]
  if (perKey) return normalizeMappingBillingObject(perKey)
  if (key === hint.mapping_key) return normalizeMappingBillingObject(hint.billing_object)
  return 'requested'
}

function itemHints(item: ModelPricingItem): BillingBasisHint[] {
  if (item.billing_basis_hints && item.billing_basis_hints.length > 0) {
    return item.billing_basis_hints
  }
  return item.billing_basis_hint ? [item.billing_basis_hint] : []
}

export function deriveModelNameRows(item: ModelPricingItem): ModelNameRow[] {
  const hints = itemHints(item)
  const entryHints = hints.filter((h) => h.mapping_target)
  if (entryHints.length > 0) {
    return entryHints.map((h) => {
      const from = h.mapping_key || item.model
      return {
        rowKey: `${h.platform || ''}:${from.toLowerCase()}`,
        mappingFrom: from,
        upstreamDisplay: h.mapping_target as string,
        platform: h.platform || '',
        isMappingEntry: true,
        billingObject: billingObjectForKey(h, from),
        mappedFrom: h.mapped_from ?? [],
      }
    })
  }

  const upstreamHint = hints.find((h) => (h.mapped_from?.length ?? 0) > 0)
  return [
    {
      rowKey: `passthrough:${item.model.toLowerCase()}`,
      mappingFrom: item.model,
      upstreamDisplay: item.model,
      platform: upstreamHint?.platform || '',
      isMappingEntry: false,
      billingObject: 'requested',
      mappedFrom: upstreamHint?.mapped_from ?? [],
    },
  ]
}
