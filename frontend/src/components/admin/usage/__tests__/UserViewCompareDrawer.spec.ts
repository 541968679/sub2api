import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserViewCompareDrawer from '../UserViewCompareDrawer.vue'
import { adminUsageAPI } from '@/api/admin/usage'

const messages: Record<string, string> = {
  'common.loading': 'Loading',
  'admin.usage.userViewCompareTitle': 'User Perspective Comparison',
  'admin.usage.userViewConfigUsed': 'Effective Display Config',
  'admin.usage.userViewSourceOverride': 'With user override',
  'admin.usage.userViewSourceGlobal': 'Global only',
  'admin.usage.userViewConfigHint': 'Display config hint',
  'admin.usage.userViewSectionTokens': 'Tokens',
  'admin.usage.userViewSectionCosts': 'Costs',
  'admin.usage.userViewSectionInvariants': 'Invariants',
  'admin.usage.userViewField': 'Field',
  'admin.usage.userViewColReal': 'Real',
  'admin.usage.userViewColUser': 'User display',
  'admin.usage.userViewDiff': 'Diff',
  'admin.usage.userViewTotal': 'Total',
  'admin.usage.userViewActualCost': 'Actual cost',
  'admin.usage.userViewRateMultiplier': 'Rate',
  'admin.usage.userViewGroupRate': 'Group rate',
  'admin.usage.userViewEmptySection': 'No data',
  'admin.usage.userViewActualCostMismatch': 'Actual mismatch',
  'admin.usage.userViewCalculationTitle': 'Cost calculation process',
  'admin.usage.userViewCalculationHint': 'Calculation hint',
  'admin.usage.userViewFormulaRealTitle': 'Real billing layer',
  'admin.usage.userViewFormulaUserTitle': 'User display layer',
  'admin.usage.userViewFormulaItem': 'Item',
  'admin.usage.userViewFormulaTokens': 'Tokens',
  'admin.usage.userViewFormulaUnitPrice': 'Unit price',
  'admin.usage.userViewFormulaCost': 'Cost',
  'admin.usage.userViewFormulaSubtotal': 'Token component subtotal',
  'admin.usage.userViewFormulaOther': 'Other / non-token cost',
  'admin.usage.userViewFormulaTotal': 'total_cost',
  'admin.usage.userViewFormulaBilled': 'total_cost x rate',
  'admin.usage.userViewFormulaActual': 'actual_cost',
  'admin.usage.userViewFormulaDiff': 'Difference',
  'admin.modelPricing.displayInputPrice': 'Display input price',
  'admin.modelPricing.displayOutputPrice': 'Display output price',
  'admin.modelPricing.displayCacheReadPrice': 'Display cache read price',
  'admin.modelPricing.displayCacheCreationPrice': 'Display cache creation price',
  'admin.modelPricing.displayCacheCreation1hPrice': 'Display 1h cache creation price',
  'admin.modelPricing.displayRateMultiplier': 'Display rate',
  'usage.input': 'Input',
  'usage.output': 'Output',
  'usage.cacheRead': 'Cache read',
  'usage.cacheCreation': 'Cache creation',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    getUserViewPreview: vi.fn(),
  },
}))

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<section v-if="show"><h1>{{ title }}</h1><slot /></section>',
}

describe('UserViewCompareDrawer', () => {
  it('shows real and display cost calculation processes', async () => {
    vi.mocked(adminUsageAPI.getUserViewPreview).mockResolvedValue({
      log_id: 16744,
      user_id: 10,
      model: 'claude-fable-5',
      real: {
        input_tokens: 2,
        output_tokens: 30,
        cache_read_tokens: 28041,
        cache_creation_tokens: 0,
        input_cost: 0.000020,
        output_cost: 0.001500,
        cache_read_cost: 0.028041,
        cache_creation_cost: 0,
        total_cost: 0.029561,
        actual_cost: 0.059122,
        rate_multiplier: 2,
      },
      user_view: {
        input_tokens: 701,
        output_tokens: 38,
        cache_read_tokens: 28041,
        cache_creation_tokens: 0,
        input_cost: 0.007010,
        output_cost: 0.001900,
        cache_read_cost: 0.028041,
        cache_creation_cost: 0,
        total_cost: 0.036951,
        actual_cost: 0.059122,
        rate_multiplier: 1.6,
        display_input_price: 10e-6,
        display_output_price: 50e-6,
        display_cache_read_price: 1e-6,
      },
      config_used: {
        display_input_price: 10e-6,
        display_output_price: 50e-6,
        display_cache_read_price: 1e-6,
        display_cache_creation_price: null,
        display_cache_creation_1h_price: null,
        display_rate_multiplier: null,
        user_group_rate: 1.6,
        has_user_override: false,
        group_id: 1,
      },
    })

    const wrapper = mount(UserViewCompareDrawer, {
      props: {
        open: true,
        logId: 16744,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
        },
      },
    })

    await flushPromises()

    const text = wrapper.text()
    expect(adminUsageAPI.getUserViewPreview).toHaveBeenCalledWith(16744)
    expect(text).toContain('Cost calculation process')
    expect(text).toContain('Real billing layer')
    expect(text).toContain('User display layer')
    expect(text).toContain('701 x $10.0000 / MTok')
    expect(text).toContain('28,041 x $1.0000 / MTok')
    expect(text).toContain('$0.007010')
    expect(text).toContain('$0.028041')
    expect(text).toContain('$0.036951')
    expect(text).toContain('$0.059122')
  })
})
