import { describe, expect, it } from 'vitest'
import { PAYMENT_CURRENCY_OPTIONS, PROVIDER_CONFIG_FIELDS } from '@/components/payment/providerConfig'

function findField(providerKey: string, key: string) {
  const fields = PROVIDER_CONFIG_FIELDS[providerKey] || []
  return fields.find(field => field.key === key)
}

describe('PROVIDER_CONFIG_FIELDS.wxpay', () => {
  it('keeps admin form validation aligned with backend-required credentials', () => {
    expect(findField('wxpay', 'publicKeyId')?.optional).toBeFalsy()
    expect(findField('wxpay', 'certSerial')?.optional).toBeFalsy()
  })

  it('exposes optional scene-specific WeChat fields without making them required', () => {
    expect(findField('wxpay', 'mpAppId')?.optional).toBe(true)
    expect(findField('wxpay', 'h5AppName')?.optional).toBe(true)
    expect(findField('wxpay', 'h5AppUrl')?.optional).toBe(true)
  })
})

describe.each(['airwallex', 'stripe'])('PROVIDER_CONFIG_FIELDS.%s', (providerKey) => {
  it('adds currency config with CNY as the default', () => {
    const currency = findField(providerKey, 'currency')
    expect(currency?.defaultValue).toBe('CNY')
    expect(currency?.hintKey).toBe('admin.settings.payment.field_paymentCurrencyHint')
    expect(currency?.options).toBe(PAYMENT_CURRENCY_OPTIONS)
  })
})

describe('PROVIDER_CONFIG_FIELDS.airwallex', () => {
  it('keeps account and environment fields configurable', () => {
    expect(findField('airwallex', 'accountId')?.optional).toBe(true)
    expect(findField('airwallex', 'apiBase')?.hintKey).toBe('admin.settings.payment.field_airwallexApiBaseHint')
  })
})
