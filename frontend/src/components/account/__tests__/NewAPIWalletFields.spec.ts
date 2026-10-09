import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'

const { listNewAPIWalletSourcesMock } = vi.hoisted(() => ({
  listNewAPIWalletSourcesMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listNewAPIWalletSources: listNewAPIWalletSourcesMock
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import NewAPIWalletFields from '../NewAPIWalletFields.vue'

const donors = [
  {
    id: 1773,
    name: 'zzs国模glm',
    platform: 'openai',
    origin: 'https://zzshu.cc',
    user_id: '8828'
  },
  {
    id: 1800,
    name: 'other wallet',
    platform: 'openai',
    origin: 'https://zzshu.cc',
    user_id: '100'
  },
  {
    id: 42,
    name: 'elsewhere',
    platform: 'anthropic',
    origin: 'https://other.example',
    user_id: '7'
  }
]

function mountFields(initial?: { baseUrl?: string; userId?: string; hasSavedToken?: boolean }) {
  const userId = ref(initial?.userId ?? '')
  const accessToken = ref('')
  const sourceAccountId = ref<number | null>(null)
  const baseUrl = ref(initial?.baseUrl ?? 'https://zzshu.cc/v1')
  const hasSavedToken = ref(!!initial?.hasSavedToken)
  const Harness = defineComponent({
    components: { NewAPIWalletFields },
    setup() {
      return { userId, accessToken, sourceAccountId, baseUrl, hasSavedToken }
    },
    template: `
      <NewAPIWalletFields
        v-model:user-id="userId"
        v-model:access-token="accessToken"
        v-model:source-account-id="sourceAccountId"
        :base-url="baseUrl"
        :has-saved-token="hasSavedToken"
      />
    `
  })
  return {
    wrapper: mount(Harness),
    userId,
    accessToken,
    sourceAccountId,
    baseUrl
  }
}

describe('NewAPIWalletFields', () => {
  it('asks for a choice when the same site has more than one wallet', async () => {
    listNewAPIWalletSourcesMock.mockResolvedValue(donors)
    const { wrapper, userId, sourceAccountId } = mountFields()
    await flushPromises()

    expect(userId.value).toBe('')
    expect(wrapper.get('[data-testid="newapi-wallet-source-hint"]').attributes('data-match')).toBe(
      'ambiguous'
    )
    expect(wrapper.findAll('[data-testid="newapi-wallet-source"] option')).toHaveLength(4)

    await wrapper.get('[data-testid="newapi-wallet-source"]').setValue('42')
    await flushPromises()
    expect(userId.value).toBe('7')
    expect(sourceAccountId.value).toBe(42)
    expect(wrapper.get('[data-testid="newapi-wallet-source-hint"]').attributes('data-match')).toBe(
      'manual'
    )
  })

  it('does not replace a saved wallet', async () => {
    listNewAPIWalletSourcesMock.mockResolvedValue([donors[0]])
    const { wrapper, userId, sourceAccountId } = mountFields({
      hasSavedToken: true,
      userId: '1'
    })
    await flushPromises()
    expect(userId.value).toBe('1')
    expect(sourceAccountId.value).toBeNull()
    expect(wrapper.find('[data-testid="newapi-wallet-source-hint"]').exists()).toBe(false)
  })

  it('auto-matches again after the base URL origin changes', async () => {
    listNewAPIWalletSourcesMock.mockResolvedValue([donors[0]])
    const { wrapper, baseUrl, userId, sourceAccountId } = mountFields({
      baseUrl: 'https://empty.example'
    })
    await flushPromises()
    expect(userId.value).toBe('')

    baseUrl.value = 'https://zzshu.cc'
    await flushPromises()
    expect(userId.value).toBe('8828')
    expect(sourceAccountId.value).toBe(1773)
    expect(wrapper.get('[data-testid="newapi-wallet-source-hint"]').attributes('data-match')).toBe(
      'auto'
    )
  })
})
