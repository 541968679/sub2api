<template>
  <div class="card p-6">
    <div class="mb-4 flex items-center gap-3">
      <div
        class="flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-900/25 dark:text-emerald-300"
      >
        <Icon name="gift" size="md" />
      </div>
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('payment.inlineRedeemTitle') }}
        </h2>
        <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
          {{ t('payment.inlineRedeemHint') }}
        </p>
      </div>
    </div>

    <form class="space-y-4" @submit.prevent="handleRedeem">
      <div>
        <label :for="inputId" class="input-label">{{ t('redeem.redeemCodeLabel') }}</label>
        <div class="relative mt-1">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
            <Icon name="gift" size="md" class="text-gray-400 dark:text-dark-500" />
          </div>
          <input
            :id="inputId"
            v-model="code"
            type="text"
            required
            autocomplete="off"
            :placeholder="t('redeem.redeemCodePlaceholder')"
            :disabled="submitting"
            class="input py-3 pl-12 text-base"
          />
        </div>
      </div>

      <button
        type="submit"
        class="btn btn-primary w-full py-3"
        :disabled="!code.trim() || submitting"
      >
        <span v-if="submitting" class="flex items-center justify-center gap-2">
          <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
          {{ t('redeem.redeeming') }}
        </span>
        <span v-else class="flex items-center justify-center gap-2">
          <Icon name="checkCircle" size="md" />
          {{ t('redeem.redeemButton') }}
        </span>
      </button>
    </form>

    <div
      v-if="successMessage"
      class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-800/50 dark:bg-emerald-900/20 dark:text-emerald-300"
    >
      {{ successMessage }}
    </div>
    <div
      v-if="errorMessage"
      class="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/50 dark:bg-red-900/20 dark:text-red-300"
    >
      {{ errorMessage }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemAPI } from '@/api/redeem'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(
  defineProps<{
    /** Unique id suffix so recharge/subscription tabs do not clash */
    instanceId?: string
  }>(),
  { instanceId: 'default' }
)

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()

const code = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const inputId = computed(() => `inline-redeem-code-${props.instanceId}`)

async function handleRedeem() {
  const trimmed = code.value.trim()
  if (!trimmed) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    const result = await redeemAPI.redeem(trimmed)
    await authStore.refreshUser()

    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true)
      } catch {
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    if (result.type === 'balance') {
      successMessage.value = `${t('redeem.added')}: $${result.value.toFixed(2)}`
    } else if (result.type === 'concurrency') {
      successMessage.value = `${t('redeem.added')}: ${result.value} ${t('redeem.concurrentRequests')}`
    } else if (result.type === 'subscription') {
      const name = result.group?.name
      successMessage.value = name
        ? t('redeem.subscriptionAssignedDesc', { groupName: name })
        : t('redeem.subscriptionRedeemed')
    } else {
      successMessage.value = result.message || t('redeem.codeRedeemSuccess')
    }

    code.value = ''
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
    emit('success')
  } catch (error: unknown) {
    const err = error as { response?: { data?: { detail?: string } } }
    errorMessage.value = err.response?.data?.detail || t('redeem.failedToRedeem')
    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}
</script>
