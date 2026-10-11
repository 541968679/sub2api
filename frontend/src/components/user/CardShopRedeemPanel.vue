<template>
  <div class="mx-auto max-w-3xl space-y-5" data-test="card-shop-redeem-panel">
    <div>
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('payment.cardShop.pageTitle') }}
      </h2>
      <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-gray-400">
        {{ t('payment.cardShop.pageDesc') }}
      </p>
    </div>

    <div class="card p-6">
      <div class="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-4">
          <div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-300">
            <Icon name="userCircle" size="lg" />
          </div>
          <div class="min-w-0">
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('payment.rechargeAccount') }}</p>
            <p class="mt-1 truncate text-xl font-semibold text-gray-900 dark:text-white">{{ user?.username || '' }}</p>
          </div>
        </div>
        <div class="rounded-lg border border-gray-100 bg-gray-50 px-5 py-4 text-left dark:border-dark-700 dark:bg-dark-800/60 sm:text-right">
          <p class="text-xs font-medium uppercase text-gray-400 dark:text-gray-500">{{ t('payment.currentBalance') }}</p>
          <p class="mt-1 text-3xl font-bold text-green-600 dark:text-green-400">${{ user?.balance?.toFixed(2) || '0.00' }}</p>
        </div>
      </div>

      <div
        v-if="activeSubscriptions.length > 0"
        class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700"
      >
        <p class="text-xs font-medium uppercase text-gray-400 dark:text-gray-500">
          {{ t('payment.activeSubscription') }}
        </p>
        <div class="mt-2 flex flex-wrap gap-2">
          <span
            v-for="sub in activeSubscriptions"
            :key="sub.id"
            class="rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
          >
            {{ sub.group?.name || t('payment.groupFallback', { id: sub.group_id }) }}
          </span>
        </div>
      </div>
    </div>

    <CardShopBuyCard :notice="notice" />

    <div class="relative">
      <div class="mb-3 flex items-center gap-3 px-1">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-sm font-bold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200">
          2
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('payment.cardShop.stepRedeemLead') }}</p>
      </div>
      <InlineRedeemCard
        instance-id="card-shop"
        :title="t('payment.cardShop.stepRedeem')"
        :hint="t('payment.cardShop.stepRedeemDesc')"
      />
    </div>

    <div class="card border-primary-200 bg-primary-50 dark:border-primary-800/50 dark:bg-primary-900/20">
      <div class="p-6">
        <h3 class="text-sm font-semibold text-primary-800 dark:text-primary-300">
          {{ t('payment.cardShop.rulesTitle') }}
        </h3>
        <ul class="mt-2 list-inside list-disc space-y-1 text-sm text-primary-700 dark:text-primary-400">
          <li>{{ t('redeem.codeRule1') }}</li>
          <li>{{ t('redeem.codeRule2') }}</li>
          <li>{{ t('redeem.codeRule4') }}</li>
        </ul>
        <router-link
          to="/redeem"
          class="mt-4 inline-flex text-sm font-medium text-primary-700 underline-offset-4 hover:underline dark:text-primary-300"
        >
          {{ t('payment.cardShop.viewHistory') }}
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import CardShopBuyCard from '@/components/user/CardShopBuyCard.vue'
import InlineRedeemCard from '@/components/user/InlineRedeemCard.vue'
import Icon from '@/components/icons/Icon.vue'

defineOptions({ name: 'CardShopRedeemPanel' })

withDefaults(defineProps<{
  notice?: string
}>(), {
  notice: '',
})

const { t } = useI18n()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)
</script>
