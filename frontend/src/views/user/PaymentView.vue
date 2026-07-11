<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>
      <template v-else>
        <!-- Payment in progress (shared by recharge and subscription) -->
        <template v-if="paymentPhase === 'paying'">
          <PaymentStatusPanel
            :order-id="paymentState.orderId"
            :qr-code="paymentState.qrCode"
            :expires-at="paymentState.expiresAt"
            :payment-type="paymentState.paymentType"
            :pay-url="paymentState.payUrl"
            :order-type="paymentState.orderType"
            :currency="paymentState.currency || selectedCurrency"
            @done="onPaymentDone"
            @success="onPaymentSuccess"
            @settled="onPaymentSettled"
          />
        </template>
        <template v-else>
          <div
            v-if="tabs.length > 1"
            class="grid grid-cols-2 gap-2 rounded-xl border border-gray-200 bg-white p-1.5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
          >
            <button
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              class="flex min-h-[52px] items-center justify-center gap-2 rounded-lg px-4 text-sm font-semibold transition-all"
              :class="activeTab === tab.key
                ? 'bg-gray-900 text-white shadow-sm dark:bg-white dark:text-gray-950'
                : 'text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-gray-100'"
              @click="switchTab(tab.key)"
            >
              <span>{{ tab.label }}</span>
              <span
                v-if="tab.key === 'subscription' && checkout.plans.length > 0"
                class="rounded-full px-2 py-0.5 text-xs"
                :class="activeTab === tab.key ? 'bg-white/15 text-white dark:text-gray-950' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300'"
              >
                {{ checkout.plans.length }}
              </span>
            </button>
          </div>

          <div v-if="activeTab === 'recharge' && !checkout.balance_disabled" class="space-y-6">
            <div class="mx-auto max-w-3xl space-y-5">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.tabTopUp') }}</h2>
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
                  v-if="checkout.cny_per_usd > 0"
                  class="mt-5 flex items-center gap-3 rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm font-medium text-blue-700 dark:border-blue-900/50 dark:bg-blue-950/30 dark:text-blue-300"
                >
                  <Icon name="dollar" size="sm" class="shrink-0" />
                  <span>{{ t('payment.rechargeRatioLabel') }}: ¥{{ checkout.cny_per_usd }} = $1</span>
                </div>
              </div>

              <div v-if="checkout.first_recharge_eligible" class="first-recharge-banner">
                <div class="first-recharge-glow"></div>
                <div class="relative z-10 flex items-center gap-5 p-6">
                  <div class="flex h-16 w-16 flex-shrink-0 items-center justify-center rounded-lg bg-white/20 text-yellow-100 backdrop-blur">
                    <Icon name="sparkles" size="xl" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="text-2xl font-bold text-white">{{ t('payment.firstRechargeTitle') }}</p>
                    <p class="mt-2 max-w-2xl text-sm leading-6 text-white/85">
                      {{ t('payment.firstRechargeDesc', { min: checkout.first_recharge_min_amount, bonus: checkout.first_recharge_bonus_usd }) }}
                    </p>
                  </div>
                </div>
              </div>

              <div v-if="enabledMethods.length === 0" class="card py-16 text-center">
                <p class="text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
              </div>
              <template v-else>
                <div v-if="sortedBonusTiers.length" class="card p-6">
                  <div class="mb-5 flex items-center gap-3">
                    <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-900/20 dark:text-amber-300">
                      <Icon name="gift" size="md" />
                    </div>
                    <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.bonusTiersTitle') }}</h2>
                  </div>
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <button
                      v-for="(tier, idx) in sortedBonusTiers"
                      :key="tier.min_amount"
                      type="button"
                      class="bonus-tier-card"
                      :class="[
                        bonusTierClass(idx, sortedBonusTiers.length),
                        amount === tier.min_amount ? 'bonus-tier-selected' : ''
                      ]"
                      @click="amount = tier.min_amount"
                    >
                      <div class="bonus-tier-badge" :class="bonusTierBadgeClass(idx, sortedBonusTiers.length)">
                        +${{ tier.bonus_usd }}
                      </div>
                      <p class="bonus-tier-threshold">{{ t('payment.bonusTierThreshold', { amount: tier.min_amount }) }}</p>
                    </button>
                  </div>
                </div>

                <div class="card p-6">
                  <div class="mb-5 flex items-center gap-3">
                    <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
                      <Icon name="creditCard" size="md" />
                    </div>
                    <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.amountLabel') }}</h2>
                  </div>
                  <AmountInput
                    v-model="amount"
                    :min="globalMinAmount"
                    :max="globalMaxAmount"
                  />
                  <p v-if="amountError" class="mt-3 text-sm text-amber-600 dark:text-amber-300">{{ amountError }}</p>
                </div>

                <div v-if="enabledMethods.length >= 1" class="card p-6">
                  <PaymentMethodSelector
                    :methods="methodOptions"
                    :selected="selectedMethod"
                    @select="selectedMethod = $event"
                  />
                </div>

                <div class="card p-6">
                  <div class="mb-5 flex items-center justify-between gap-4">
                    <div class="flex items-center gap-3">
                      <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-300">
                        <Icon name="calculator" size="md" />
                      </div>
                      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.totalCredit') }}</h2>
                    </div>
                    <span class="text-2xl font-bold text-green-600 dark:text-green-400">${{ totalCreditUsd.toFixed(2) }}</span>
                  </div>

                  <div v-if="validAmount > 0" class="space-y-3 text-sm">
                    <div class="flex justify-between gap-4">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('payment.paymentAmount') }}</span>
                      <span class="font-medium text-gray-900 dark:text-white">¥{{ validAmount.toFixed(2) }}</span>
                    </div>
                    <div v-if="feeRate > 0" class="flex justify-between gap-4">
                      <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
                      <span class="font-medium text-gray-900 dark:text-white">¥{{ feeAmount.toFixed(2) }}</span>
                    </div>
                    <div v-if="feeRate > 0" class="flex justify-between gap-4 border-t border-gray-200 pt-3 dark:border-dark-600">
                      <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                      <span class="text-lg font-bold text-primary-600 dark:text-primary-400">¥{{ totalAmount.toFixed(2) }}</span>
                    </div>
                    <div class="space-y-2 border-t border-gray-200 pt-3 dark:border-dark-600">
                      <div class="flex justify-between gap-4">
                        <span class="text-gray-500 dark:text-gray-400">{{ t('payment.baseCredit') }}</span>
                        <span class="font-medium text-gray-900 dark:text-white">${{ baseCreditUsd.toFixed(2) }}</span>
                      </div>
                      <div v-if="matchedBonus > 0" class="flex justify-between gap-4">
                        <span class="text-amber-600 dark:text-amber-400">{{ t('payment.bonusCredit') }}</span>
                        <span class="font-medium text-amber-600 dark:text-amber-400">+${{ matchedBonus.toFixed(2) }}</span>
                      </div>
                      <div v-if="firstRechargeBonus > 0" class="flex justify-between gap-4">
                        <span class="text-pink-600 dark:text-pink-400">{{ t('payment.firstRechargeBonus') }}</span>
                        <span class="font-medium text-pink-600 dark:text-pink-400">+${{ firstRechargeBonus.toFixed(2) }}</span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="rounded-lg border border-dashed border-gray-200 px-4 py-8 text-center text-sm text-gray-400 dark:border-dark-600 dark:text-gray-500">
                    {{ t('payment.enterAmount') }}
                  </div>

                  <button :class="['btn mt-6 w-full py-3 text-base font-medium', paymentButtonClass]" :disabled="!canSubmit || submitting" @click="handleSubmitRecharge">
                    <span v-if="submitting" class="flex items-center justify-center gap-2">
                      <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                      {{ t('common.processing') }}
                    </span>
                    <span v-else>{{ t('payment.createOrder') }} ¥{{ totalAmount.toFixed(2) }}</span>
                  </button>
                </div>
              </template>
            </div>
          </div>
          <div v-if="activeTab === 'subscription'" class="space-y-6">
            <div class="flex items-center justify-between gap-4">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.tabSubscribe') }}</h2>
              <span v-if="checkout.plans.length > 0" class="rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-300">
                {{ checkout.plans.length }}
              </span>
            </div>
            <!-- Subscription confirm (inline, replaces plan list) -->
            <template v-if="selectedPlan">
              <div class="grid items-start gap-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(360px,0.9fr)]">
                <div class="card p-6">
                  <div class="mb-6 flex items-start justify-between gap-4">
                    <div class="min-w-0">
                      <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('payment.confirmSubscription') }}</p>
                      <h3 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ selectedPlan.name }}</h3>
                    </div>
                    <div class="shrink-0 text-right">
                      <span v-if="selectedPlan.original_price" class="block text-sm text-gray-400 line-through dark:text-gray-500">
                        ¥{{ selectedPlan.original_price }}
                      </span>
                      <div class="flex items-baseline gap-1">
                        <span :class="['text-4xl font-bold', planTextClass]">¥{{ selectedPlan.price }}</span>
                        <span class="text-sm text-gray-500 dark:text-gray-400">/ {{ planValiditySuffix }}</span>
                      </div>
                    </div>
                  </div>

                  <p v-if="selectedPlan.description" class="text-sm leading-6 text-gray-500 dark:text-gray-400">
                    {{ selectedPlan.description }}
                  </p>

                  <div class="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div v-if="selectedPlan.daily_limit_usd != null" class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                      <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.dailyLimit') }}</span>
                      <div class="mt-1 text-xl font-semibold text-gray-800 dark:text-gray-200">${{ selectedPlan.daily_limit_usd }}</div>
                    </div>
                    <div v-if="selectedPlan.weekly_limit_usd != null" class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                      <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.weeklyLimit') }}</span>
                      <div class="mt-1 text-xl font-semibold text-gray-800 dark:text-gray-200">${{ selectedPlan.weekly_limit_usd }}</div>
                    </div>
                    <div v-if="selectedPlan.monthly_limit_usd != null" class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                      <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.monthlyLimit') }}</span>
                      <div class="mt-1 text-xl font-semibold text-gray-800 dark:text-gray-200">${{ selectedPlan.monthly_limit_usd }}</div>
                    </div>
                    <div v-if="selectedPlan.daily_limit_usd == null && selectedPlan.weekly_limit_usd == null && selectedPlan.monthly_limit_usd == null" class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
                      <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('payment.planCard.quota') }}</span>
                      <div class="mt-1 text-xl font-semibold text-gray-800 dark:text-gray-200">{{ t('payment.planCard.unlimited') }}</div>
                    </div>
                  </div>

                  <div v-if="selectedPlan.features.length > 0" class="mt-6">
                    <p class="mb-3 text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('payment.planFeatures') }}</p>
                    <div class="grid gap-3 sm:grid-cols-2">
                      <div v-for="feature in selectedPlan.features" :key="feature" class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300">
                        <Icon name="checkCircle" size="sm" class="mt-0.5 shrink-0 text-green-500" />
                        <span>{{ feature }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="space-y-6">
                  <div v-if="enabledMethods.length >= 1" class="card p-6">
                    <PaymentMethodSelector
                      :methods="subMethodOptions"
                      :selected="selectedMethod"
                      @select="selectedMethod = $event"
                    />
                  </div>
                  <div v-else class="card py-16 text-center">
                    <p class="text-gray-500 dark:text-gray-400">{{ t('payment.notAvailable') }}</p>
                  </div>

                  <div class="card p-6">
                    <div class="space-y-3 text-sm">
                      <div class="flex justify-between gap-4">
                        <span class="text-gray-500 dark:text-gray-400">{{ t('payment.amountLabel') }}</span>
                        <span class="font-medium text-gray-900 dark:text-white">¥{{ selectedPlan.price.toFixed(2) }}</span>
                      </div>
                      <div v-if="feeRate > 0 && selectedPlan.price > 0" class="flex justify-between gap-4">
                        <span class="text-gray-500 dark:text-gray-400">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
                        <span class="font-medium text-gray-900 dark:text-white">¥{{ subFeeAmount.toFixed(2) }}</span>
                      </div>
                      <div class="flex justify-between gap-4 border-t border-gray-200 pt-3 dark:border-dark-600">
                        <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.actualPay') }}</span>
                        <span class="text-xl font-bold text-primary-600 dark:text-primary-400">¥{{ (feeRate > 0 ? subTotalAmount : selectedPlan.price).toFixed(2) }}</span>
                      </div>
                    </div>

                    <button :class="['btn mt-6 w-full py-3 text-base font-medium', paymentButtonClass]" :disabled="!canSubmitSubscription || submitting" @click="confirmSubscribe">
                      <span v-if="submitting" class="flex items-center justify-center gap-2">
                        <span class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                        {{ t('common.processing') }}
                      </span>
                      <span v-else>{{ t('payment.createOrder') }} ¥{{ (feeRate > 0 ? subTotalAmount : selectedPlan.price).toFixed(2) }}</span>
                    </button>
                    <button class="btn btn-secondary mt-3 w-full" @click="selectedPlan = null">{{ t('common.cancel') }}</button>
                  </div>
                </div>
              </div>
            </template>
            <!-- Plan list -->
            <template v-else>
              <div v-if="activeSubscriptions.length > 0" class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-900">
                <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
                  <div class="flex shrink-0 items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                    <Icon name="badge" size="sm" class="text-green-600 dark:text-green-400" />
                    <span>{{ t('payment.activeSubscription') }}</span>
                  </div>
                  <div class="grid min-w-0 flex-1 gap-2 md:grid-cols-2 xl:grid-cols-3">
                  <div
                    v-for="sub in activeSubscriptions"
                    :key="sub.id"
                      class="flex min-w-0 items-center gap-2 rounded-md border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800/60"
                  >
                      <div :class="['h-7 w-1 shrink-0 rounded-full', platformAccentBarClass(sub.group?.platform || '')]" />
                    <div class="min-w-0 flex-1">
                        <div class="flex min-w-0 items-center gap-1.5">
                          <span class="truncate text-xs font-semibold text-gray-900 dark:text-white">{{ sub.group?.name || t('payment.groupFallback', { id: sub.group_id }) }}</span>
                          <span class="badge badge-success shrink-0 text-[10px]">{{ t('userSubscriptions.status.active') }}</span>
                        </div>
                        <div class="mt-0.5 truncate text-[11px] text-gray-500 dark:text-gray-400">
                          <span v-if="isUnlimitedSubscription(sub)">{{ t('payment.planCard.unlimited') }} · </span>
                        <span v-if="sub.expires_at">{{ t('userSubscriptions.daysRemaining', { days: getDaysRemaining(sub.expires_at) }) }}</span>
                        <span v-else>{{ t('userSubscriptions.noExpiration') }}</span>
                      </div>
                    </div>
                  </div>
                </div>
                </div>
              </div>

              <div v-if="checkout.plans.length === 0" class="card py-16 text-center">
                <Icon name="gift" size="xl" class="mx-auto mb-3 text-gray-300 dark:text-dark-600" />
                <p class="text-gray-500 dark:text-gray-400">{{ t('payment.noPlans') }}</p>
              </div>
              <div v-else :class="planGridClass">
                <SubscriptionPlanCard v-for="plan in checkout.plans" :key="plan.id" :plan="plan" :active-subscriptions="activeSubscriptions" @select="selectPlan" />
              </div>
            </template>
          </div>
        </template>
        <div v-if="(checkout.help_text || checkout.help_image_url) && paymentPhase === 'select' && !selectedPlan" class="card p-6">
          <div class="flex flex-col items-center gap-4">
            <img v-if="checkout.help_image_url" :src="checkout.help_image_url" alt=""
              class="h-48 max-w-full cursor-pointer rounded-lg object-contain transition-opacity hover:opacity-80"
              @click="previewImage = checkout.help_image_url" />
            <p v-if="checkout.help_text" class="text-center text-sm leading-6 text-gray-500 dark:text-gray-400">{{ checkout.help_text }}</p>
          </div>
        </div>
        <SupportContactBar
          v-if="paymentPhase === 'select'"
          context="payment"
          class="mx-auto max-w-3xl"
        />
      </template>
    </div>
    <!-- Renewal Plan Selection Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="showRenewalModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" @click.self="closeRenewalModal">
          <div class="relative w-full max-w-lg rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-dark-700 dark:bg-dark-900">
            <!-- Close button -->
            <button class="absolute right-4 top-4 rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200" @click="closeRenewalModal">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
            <h3 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">{{ t('payment.selectPlan') }}</h3>
            <div class="space-y-4">
              <SubscriptionPlanCard v-for="plan in renewalPlans" :key="plan.id" :plan="plan" :active-subscriptions="activeSubscriptions" @select="selectPlanFromModal" />
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
    <!-- Image Preview Overlay -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="previewImage" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 backdrop-blur-sm" @click="previewImage = ''">
          <img :src="previewImage" alt="" class="max-h-[85vh] max-w-[90vw] rounded-xl object-contain shadow-2xl" />
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import type { UserSubscription } from '@/types'
import type { SubscriptionPlan, CheckoutInfoResponse, CreateOrderResult, OrderType } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import AmountInput from '@/components/payment/AmountInput.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import { METHOD_ORDER, getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  buildCreateOrderPayload,
  clearPaymentRecoverySnapshot,
  decidePaymentLaunch,
  getVisibleMethods,
  normalizeVisibleMethod,
  readPaymentRecoverySnapshot,
  type PaymentRecoverySnapshot,
  writePaymentRecoverySnapshot,
} from '@/components/payment/paymentFlow'
import { platformAccentBarClass, platformTextClass } from '@/utils/platformColors'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import SupportContactBar from '@/components/common/SupportContactBar.vue'
import Icon from '@/components/icons/Icon.vue'
import { normalizePaymentCurrency } from '@/components/payment/currency'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { buildPaymentErrorToastMessage, describePaymentScenarioError } from './paymentUx'
import { hasWechatResumeQuery, parseWechatResumeRoute, stripWechatResumeQuery } from './paymentWechatResume'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const paymentStore = usePaymentStore()
const subscriptionStore = useSubscriptionStore()
const appStore = useAppStore()

const user = computed(() => authStore.user)
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)

function getDaysRemaining(expiresAt: string): number {
  const diff = new Date(expiresAt).getTime() - Date.now()
  return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)))
}

function isUnlimitedSubscription(subscription: UserSubscription): boolean {
  const group = subscription.group
  return group?.daily_limit_usd == null && group?.weekly_limit_usd == null && group?.monthly_limit_usd == null
}

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const errorHintMessage = ref('')
const activeTab = ref<'recharge' | 'subscription'>('recharge')
const amount = ref<number | null>(null)
const selectedMethod = ref('')
const selectedPlan = ref<SubscriptionPlan | null>(null)
const previewImage = ref('')

const paymentPhase = ref<'select' | 'paying'>('select')

interface CreateOrderOptions {
  openid?: string
  wechatResumeToken?: string
  paymentType?: string
  isResume?: boolean
  mobileQrFallbackAttempted?: boolean
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (result: Record<string, unknown>) => void,
  ): void
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    createdAt: 0,
  }
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike }).WeixinJSBridge
}

function waitForWeixinJSBridge(timeoutMs = 4000): Promise<WeixinJSBridgeLike | null> {
  const existing = getWeixinJSBridge()
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let settled = false
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return
      settled = true
      document.removeEventListener('WeixinJSBridgeReady', handleReady)
      document.removeEventListener('onWeixinJSBridgeReady', handleReady)
      window.clearTimeout(timer)
      resolve(bridge)
    }
    const handleReady = () => finish(getWeixinJSBridge() ?? null)
    const timer = window.setTimeout(() => finish(getWeixinJSBridge() ?? null), timeoutMs)
    document.addEventListener('WeixinJSBridgeReady', handleReady, false)
    document.addEventListener('onWeixinJSBridgeReady', handleReady, false)
  })
}

async function invokeWechatJsapiPayment(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  const bridge = await waitForWeixinJSBridge()
  if (!bridge) {
    throw new Error('WECHAT_JSAPI_UNAVAILABLE')
  }
  return new Promise((resolve) => {
    bridge.invoke('getBrandWCPayRequest', payload, (result) => resolve(result || {}))
  })
}

const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())

function persistRecoverySnapshot(snapshot: PaymentRecoverySnapshot) {
  if (typeof window === 'undefined' || !snapshot.orderId) return
  writePaymentRecoverySnapshot(window.localStorage, snapshot, PAYMENT_RECOVERY_STORAGE_KEY)
}

function removeRecoverySnapshot() {
  if (typeof window === 'undefined') return
  clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}

function resetPayment() {
  paymentPhase.value = 'select'
  paymentState.value = emptyPaymentState()
  removeRecoverySnapshot()
}

async function redirectToPaymentResult(state: PaymentRecoverySnapshot): Promise<void> {
  const query: Record<string, string | undefined> = {}
  if (state.orderId > 0) {
    query.order_id = String(state.orderId)
  }
  if (state.outTradeNo) {
    query.out_trade_no = state.outTradeNo
  }
  if (state.resumeToken) {
    query.resume_token = state.resumeToken
  }
  await router.push({
    path: '/payment/result',
    query,
  })
}

function buildWechatOAuthAuthorizeUrl(
  authorizeUrl: string,
  context: { paymentType: string; orderType: OrderType; planId?: number; orderAmount: number },
): string {
  const normalizedUrl = authorizeUrl.trim()
  if (!normalizedUrl || typeof window === 'undefined') {
    return normalizedUrl
  }

  try {
    const targetUrl = new URL(normalizedUrl, window.location.origin)
    const redirectPath = targetUrl.searchParams.get('redirect') || '/purchase'
    const redirectUrl = new URL(redirectPath, window.location.origin)
    const paymentType = normalizeVisibleMethod(context.paymentType) || context.paymentType.trim() || 'wxpay'

    redirectUrl.searchParams.set('payment_type', paymentType)
    redirectUrl.searchParams.set('order_type', context.orderType)

    if (context.planId) {
      redirectUrl.searchParams.set('plan_id', String(context.planId))
    } else {
      redirectUrl.searchParams.delete('plan_id')
    }

    if (context.orderAmount > 0) {
      redirectUrl.searchParams.set('amount', String(context.orderAmount))
    } else {
      redirectUrl.searchParams.delete('amount')
    }

    targetUrl.searchParams.set('redirect', `${redirectUrl.pathname}${redirectUrl.search}`)
    return targetUrl.toString()
  } catch {
    return normalizedUrl
  }
}

function onPaymentDone() {
  const wasSubscription = paymentState.value.orderType === 'subscription'
  resetPayment()
  selectedPlan.value = null
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
}

function onPaymentSuccess() {
  removeRecoverySnapshot()
  authStore.refreshUser()
  if (paymentState.value.orderType === 'subscription') {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
}

function onPaymentSettled() {
  removeRecoverySnapshot()
}

// All checkout data from single API call
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0,
  plans: [], balance_disabled: false, balance_recharge_multiplier: 1, recharge_fee_rate: 0, cny_per_usd: 0, bonus_tiers: [], help_text: '', help_image_url: '', stripe_publishable_key: '',
  first_recharge_enabled: false, first_recharge_min_amount: 0, first_recharge_bonus_usd: 0, first_recharge_eligible: false,
})

const tabs = computed(() => {
  const result: { key: 'recharge' | 'subscription'; label: string }[] = []
  if (!checkout.value.balance_disabled) result.push({ key: 'recharge', label: t('payment.tabTopUp') })
  result.push({ key: 'subscription', label: t('payment.tabSubscribe') })
  return result
})

const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
const enabledMethods = computed(() => Object.keys(visibleMethods.value))
const validAmount = computed(() => amount.value ?? 0)
const balanceRechargeMultiplier = computed(() => {
  const multiplier = checkout.value.balance_recharge_multiplier
  return multiplier > 0 ? multiplier : 1
})

// Adaptive grid: center single card, 2-col for 2 plans, 3-col for 3+
const planGridClass = computed(() => {
  return 'grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3'
})

// Check if an amount fits a method's [min, max]. 0 = no limit.
function amountFitsMethod(amt: number, methodType: string): boolean {
  if (amt <= 0) return true
  const ml = visibleMethods.value[methodType]
  if (!ml) return false
  if (ml.single_min > 0 && amt < ml.single_min) return false
  if (ml.single_max > 0 && amt > ml.single_max) return false
  return true
}

// Visible methods decide the amount range shown to users.
const globalMinAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_min <= 0)) return 0
  return Math.min(...limits.map(limit => limit.single_min))
})
const globalMaxAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_max <= 0)) return 0
  return Math.max(...limits.map(limit => limit.single_max))
})

// Selected method's limits (for validation and error messages)
const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))

const methodOptions = computed<PaymentMethodOption[]>(() =>
  enabledMethods.value.map((type) => {
    const ml = visibleMethods.value[type]
    return {
      type,
      display_name: ml?.display_name,
      fee_rate: ml?.fee_rate ?? 0,
      available: ml?.available !== false && amountFitsMethod(validAmount.value, type),
    }
  })
)

const feeRate = computed(() => checkout.value?.recharge_fee_rate ?? 0)

const sortedBonusTiers = computed(() =>
  [...(checkout.value?.bonus_tiers || [])].sort((a, b) => a.min_amount - b.min_amount)
)

const baseCreditUsd = computed(() => {
  const rate = checkout.value?.cny_per_usd
  if (!rate || rate <= 0 || validAmount.value <= 0) return 0
  return Math.round((validAmount.value / rate) * balanceRechargeMultiplier.value * 100) / 100
})

const matchedBonus = computed(() => {
  if (validAmount.value <= 0 || !sortedBonusTiers.value.length) return 0
  let bonus = 0
  for (const tier of sortedBonusTiers.value) {
    if (validAmount.value >= tier.min_amount) {
      bonus = tier.bonus_usd
    }
  }
  return bonus
})

const firstRechargeBonus = computed(() => {
  if (!checkout.value.first_recharge_eligible ||
      checkout.value.first_recharge_bonus_usd <= 0 ||
      validAmount.value < checkout.value.first_recharge_min_amount) {
    return 0
  }
  return checkout.value.first_recharge_bonus_usd
})

const totalBonus = computed(() => matchedBonus.value + firstRechargeBonus.value)

const totalCreditUsd = computed(() =>
  Math.round((baseCreditUsd.value + totalBonus.value) * 100) / 100
)
const feeAmount = computed(() =>
  feeRate.value > 0 && validAmount.value > 0
    ? Math.ceil(((validAmount.value * feeRate.value) / 100) * 100) / 100
    : 0
)
const totalAmount = computed(() =>
  feeRate.value > 0 && validAmount.value > 0
    ? Math.round((validAmount.value + feeAmount.value) * 100) / 100
    : validAmount.value
)

const amountError = computed(() => {
  if (validAmount.value <= 0) return ''
  // No method can handle this amount
  if (!enabledMethods.value.some((m) => amountFitsMethod(validAmount.value, m))) {
    return t('payment.amountNoMethod')
  }
  // Selected method can't handle this amount (but others can)
  const ml = selectedLimit.value
  if (ml) {
    if (ml.single_min > 0 && validAmount.value < ml.single_min) return t('payment.amountTooLow', { min: ml.single_min })
    if (ml.single_max > 0 && validAmount.value > ml.single_max) return t('payment.amountTooHigh', { max: ml.single_max })
  }
  return ''
})

const canSubmit = computed(() =>
  validAmount.value > 0
    && amountFitsMethod(validAmount.value, selectedMethod.value)
    && selectedLimit.value?.available !== false
)

// Subscription-specific: method options based on plan price
const subMethodOptions = computed<PaymentMethodOption[]>(() => {
  const planPrice = selectedPlan.value?.price ?? 0
  return enabledMethods.value.map((type) => {
    const ml = visibleMethods.value[type]
    return {
      type,
      display_name: ml?.display_name,
      fee_rate: ml?.fee_rate ?? 0,
      available: ml?.available !== false && amountFitsMethod(planPrice, type),
    }
  })
})

const subFeeAmount = computed(() => {
  const price = selectedPlan.value?.price ?? 0
  if (feeRate.value <= 0 || price <= 0) return 0
  return Math.ceil(((price * feeRate.value) / 100) * 100) / 100
})

const subTotalAmount = computed(() => {
  const price = selectedPlan.value?.price ?? 0
  if (feeRate.value <= 0 || price <= 0) return price
  return Math.round((price + subFeeAmount.value) * 100) / 100
})

const canSubmitSubscription = computed(() =>
  selectedPlan.value !== null
    && amountFitsMethod(selectedPlan.value.price, selectedMethod.value)
    && selectedLimit.value?.available !== false
)

// Auto-switch to first available method when current selection can't handle the amount
watch(() => [validAmount.value, selectedMethod.value] as const, ([amt, method]) => {
  if (amt <= 0 || amountFitsMethod(amt, method)) return
  const available = enabledMethods.value.find((m) => amountFitsMethod(amt, m))
  if (available) selectedMethod.value = available
})

// Payment button class: follows selected payment method color
const paymentButtonClass = computed(() => {
  const m = selectedMethod.value
  if (!m) return 'btn-primary'
  if (isBuiltInAlipayMethod(m)) return 'btn-alipay'
  if (isBuiltInWxpayMethod(m)) return 'btn-wxpay'
  if (m === 'stripe') return 'btn-stripe'
  if (m === 'airwallex') return 'btn-airwallex'
  return 'btn-primary'
})

// Subscription confirm: platform accent colors (clean card, no gradient)
const planTextClass = computed(() => platformTextClass(selectedPlan.value?.group_platform || ''))

function switchTab(tab: 'recharge' | 'subscription') {
  activeTab.value = tab
  if (tab === 'recharge') {
    selectedPlan.value = null
  }
}

// Renewal modal state
const showRenewalModal = ref(false)
const renewGroupId = ref<number | null>(null)
const renewalPlans = computed(() => {
  if (renewGroupId.value == null) return []
  return checkout.value.plans.filter(p => p.group_id === renewGroupId.value)
})

const planValiditySuffix = computed(() => {
  if (!selectedPlan.value) return ''
  const u = selectedPlan.value.validity_unit || 'day'
  if (u === 'month') return t('payment.perMonth')
  if (u === 'year') return t('payment.perYear')
  return `${selectedPlan.value.validity_days}${t('payment.days')}`
})

function selectPlan(plan: SubscriptionPlan) {
  selectedPlan.value = plan
  errorMessage.value = ''
}

function selectPlanFromModal(plan: SubscriptionPlan) {
  showRenewalModal.value = false
  renewGroupId.value = null
  selectedPlan.value = plan
  errorMessage.value = ''
}

function closeRenewalModal() {
  showRenewalModal.value = false
  renewGroupId.value = null
}

async function handleSubmitRecharge() {
  if (!canSubmit.value || submitting.value) return
  await createOrder(validAmount.value, 'balance')
}

async function confirmSubscribe() {
  if (!selectedPlan.value || submitting.value) return
  await createOrder(selectedPlan.value.price, 'subscription', selectedPlan.value.id)
}

async function createOrder(orderAmount: number, orderType: OrderType, planId?: number, options: CreateOrderOptions = {}) {
  submitting.value = true
  errorMessage.value = ''
  errorHintMessage.value = ''
  const requestType = normalizeVisibleMethod(options.paymentType || selectedMethod.value) || options.paymentType || selectedMethod.value
  try {
    const payload = buildCreateOrderPayload({
      amount: orderAmount,
      paymentType: requestType,
      orderType,
      planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
    })
    if (options.openid) {
      payload.openid = options.openid
    }
    if (options.wechatResumeToken) {
      payload.wechat_resume_token = options.wechatResumeToken
    }

    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const openWindow = (url: string) => {
      const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
      if (!win || win.closed) {
        window.location.href = url
      }
    }
    const visibleMethod = normalizeVisibleMethod(requestType) || requestType
    // When user clicks the dedicated Stripe button, leave method blank so the
    // landing page renders Stripe's full Payment Element (card/link/alipay/wxpay).
    const stripeMethod = visibleMethod === 'stripe'
      ? ''
      : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret && visibleMethod !== 'airwallex'
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const airwallexRouteUrl = result.client_secret && result.intent_id
      ? router.resolve({
        path: '/payment/airwallex',
        query: {
          order_id: String(result.order_id),
          out_trade_no: result.out_trade_no || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType,
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
      airwallexRouteUrl,
    })

    if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
      window.location.href = buildWechatOAuthAuthorizeUrl(decision.oauth.authorize_url, {
        paymentType: visibleMethod,
        orderType,
        planId,
        orderAmount,
      })
      return
    }

    if (decision.kind === 'unhandled') {
      applyScenarioError({ reason: 'UNHANDLED_PAYMENT_SCENARIO' }, visibleMethod)
      return
    }

    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)

    if (decision.kind === 'stripe_popup') {
      openWindow(decision.paymentState.payUrl)
      return
    }
    if (decision.kind === 'stripe_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'airwallex_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'wechat_jsapi' && decision.jsapi) {
      try {
        const jsapiResult = await invokeWechatJsapiPayment(decision.jsapi as Record<string, unknown>)
        const errMsg = String(jsapiResult.err_msg || '').toLowerCase()
        if (errMsg.includes('cancel')) {
          appStore.showInfo(t('payment.qr.cancelled'))
          resetPayment()
        } else if (errMsg && !errMsg.includes('ok')) {
          resetPayment()
          const fallbackApplied = await attemptMobileQrFallback(
            { reason: 'WECHAT_JSAPI_FAILED', message: errMsg },
            {
              orderAmount,
              orderType,
              planId,
              paymentType: visibleMethod,
              attempted: options.mobileQrFallbackAttempted === true,
              wechatResumeToken: options.wechatResumeToken,
            },
          )
          if (!fallbackApplied) {
            applyScenarioError({ reason: 'WECHAT_JSAPI_FAILED', message: errMsg }, visibleMethod)
          }
        } else {
          const resultState = { ...decision.paymentState }
          resetPayment()
          await redirectToPaymentResult(resultState)
        }
      } catch (err: unknown) {
        resetPayment()
        const fallbackApplied = await attemptMobileQrFallback(err, {
          orderAmount,
          orderType,
          planId,
          paymentType: visibleMethod,
          attempted: options.mobileQrFallbackAttempted === true,
          wechatResumeToken: options.wechatResumeToken,
        })
        if (!fallbackApplied) {
          throw err
        }
      }
      return
    }
    if (decision.kind === 'redirect_waiting' && decision.paymentState.payUrl) {
      if (isMobileDevice()) {
        window.location.href = decision.paymentState.payUrl
        return
      }
      openWindow(decision.paymentState.payUrl)
    }
  } catch (err: unknown) {
    const apiErr = err as Record<string, unknown>
    if (apiErr.reason === 'TOO_MANY_PENDING') {
      const metadata = apiErr.metadata as Record<string, unknown> | undefined
      errorMessage.value = t('payment.errors.tooManyPending', { max: metadata?.max || '' })
      errorHintMessage.value = ''
    } else if (apiErr.reason === 'CANCEL_RATE_LIMITED') {
      errorMessage.value = t('payment.errors.cancelRateLimited')
      errorHintMessage.value = ''
    } else if (await attemptMobileQrFallback(err, {
      orderAmount,
      orderType,
      planId,
      paymentType: requestType,
      attempted: options.mobileQrFallbackAttempted === true,
      wechatResumeToken: options.wechatResumeToken,
    })) {
      return
    } else {
      const handled = applyScenarioError(
        err,
        normalizeVisibleMethod(options.paymentType || selectedMethod.value) || selectedMethod.value,
      )
      if (!handled) {
        errorMessage.value = extractI18nErrorMessage(err, t, 'payment.errors', extractApiErrorMessage(err, t('payment.result.failed')))
        errorHintMessage.value = ''
      }
      if (handled) {
        return
      }
    }
    appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  } finally {
    submitting.value = false
  }
}

interface MobileQrFallbackContext {
  orderAmount: number
  orderType: OrderType
  planId?: number
  paymentType: string
  attempted: boolean
  wechatResumeToken?: string
}

function shouldFallbackToDesktopQr(err: unknown, paymentMethod: string, attempted: boolean): boolean {
  if (attempted || !isMobileDevice()) {
    return false
  }

  const normalizedMethod = normalizeVisibleMethod(paymentMethod) || paymentMethod
  const reason = typeof err === 'object' && err && 'reason' in err && typeof err.reason === 'string'
    ? err.reason
    : ''
  const message = err instanceof Error
    ? err.message
    : (typeof err === 'object' && err && 'message' in err && typeof err.message === 'string'
      ? err.message
      : '')
  const normalizedMessage = message.toLowerCase()

  if (normalizedMethod === 'wxpay') {
    return reason === 'WECHAT_H5_NOT_AUTHORIZED'
      || reason === 'WECHAT_PAYMENT_MP_NOT_CONFIGURED'
      || reason === 'WECHAT_JSAPI_FAILED'
      || reason === 'PAYMENT_GATEWAY_ERROR'
      || reason === 'UNHANDLED_PAYMENT_SCENARIO'
      || normalizedMessage.includes('weixinjsbridge is unavailable')
      || normalizedMessage.includes('wechat_jsapi_unavailable')
  }

  if (normalizedMethod === 'alipay') {
    return reason === 'PAYMENT_GATEWAY_ERROR' || reason === 'UNHANDLED_PAYMENT_SCENARIO'
  }

  return false
}

async function attemptMobileQrFallback(err: unknown, context: MobileQrFallbackContext): Promise<boolean> {
  if (!shouldFallbackToDesktopQr(err, context.paymentType, context.attempted)) {
    return false
  }

  try {
    const visibleMethod = normalizeVisibleMethod(context.paymentType) || context.paymentType
    const payload = buildCreateOrderPayload({
      amount: context.orderAmount,
      paymentType: visibleMethod,
      orderType: context.orderType,
      planId: context.planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: false,
      isWechatBrowser: false,
    })
    if (visibleMethod === 'wxpay') {
      payload.force_native_qr = true
      if (context.wechatResumeToken) {
        payload.wechat_resume_token = context.wechatResumeToken
      }
    }
    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const stripeMethod = visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType: context.orderType,
      isMobile: false,
      isWechatBrowser: false,
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
    })

    if (decision.kind !== 'qr_waiting' || !decision.paymentState.qrCode) {
      return false
    }

    errorMessage.value = ''
    errorHintMessage.value = ''
    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)
    appStore.showWarning(t('payment.errors.mobilePaymentFallbackToQr'))
    return true
  } catch {
    return false
  }
}

function applyScenarioError(err: unknown, paymentMethod: string): boolean {
  const descriptor = describePaymentScenarioError(err, {
    paymentMethod,
    isMobile: isMobileDevice(),
    isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
  })
  if (!descriptor) {
    errorMessage.value = ''
    errorHintMessage.value = ''
    return false
  }
  errorMessage.value = t(descriptor.messageKey)
  errorHintMessage.value = descriptor.hintKey ? t(descriptor.hintKey) : ''
  appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
  return true
}

async function resumeWechatPaymentFromQuery() {
  const resume = parseWechatResumeRoute(route.query, checkout.value.plans, validAmount.value)
  if (!resume) {
    return
  }

  selectedMethod.value = resume.paymentType
  if (resume.orderType === 'balance' && resume.orderAmount > 0) {
    amount.value = resume.orderAmount
  }
  if (resume.orderType === 'subscription' && resume.planId) {
    selectedPlan.value = checkout.value.plans.find(plan => plan.id === resume.planId) ?? null
  }

  await router.replace({ path: route.path, query: stripWechatResumeQuery(route.query) })

  if (resume.wechatResumeToken) {
    await createOrder(0, resume.orderType, resume.planId, {
      wechatResumeToken: resume.wechatResumeToken,
      paymentType: resume.paymentType,
      isResume: true,
    })
    return
  }

  if (resume.orderAmount > 0 && resume.openid) {
    await createOrder(resume.orderAmount, resume.orderType, resume.planId, {
      openid: resume.openid,
      paymentType: resume.paymentType,
      isResume: true,
    })
  }
}

onMounted(async () => {
  try {
    const res = await paymentAPI.getCheckoutInfo()
    checkout.value = res.data
    if (enabledMethods.value.length) {
      const order: readonly string[] = METHOD_ORDER
      const sorted = [...enabledMethods.value].sort((a, b) => {
        const ai = order.indexOf(a)
        const bi = order.indexOf(b)
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      })
      selectedMethod.value = sorted[0]
    }
    if (typeof window !== 'undefined') {
      if (hasWechatResumeQuery(route.query)) {
        removeRecoverySnapshot()
      }
      const routeResumeToken = typeof route.query.resume_token === 'string'
        ? route.query.resume_token
        : typeof route.query.wechat_resume_token === 'string'
          ? route.query.wechat_resume_token
          : undefined
      const restored = readPaymentRecoverySnapshot(
        window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
        { resumeToken: routeResumeToken },
      )
      if (restored) {
        paymentState.value = restored
        paymentPhase.value = 'paying'
        const restoredMethod = normalizeVisibleMethod(restored.paymentType)
          || (visibleMethods.value[restored.paymentType] ? restored.paymentType : '')
        if (restoredMethod) {
          selectedMethod.value = restoredMethod
        }
      } else {
        removeRecoverySnapshot()
      }
    }
    await resumeWechatPaymentFromQuery()
    if (checkout.value.balance_disabled) {
      activeTab.value = 'subscription'
    }
    // Handle renewal navigation: ?tab=subscription&group=123
    if (route.query.tab === 'subscription') {
      activeTab.value = 'subscription'
      if (route.query.group) {
        const groupId = Number(route.query.group)
        const groupPlans = checkout.value.plans.filter(p => p.group_id === groupId)
        if (groupPlans.length === 1) {
          selectedPlan.value = groupPlans[0]
        } else if (groupPlans.length > 1) {
          renewGroupId.value = groupId
          showRenewalModal.value = true
        }
      }
    }
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { loading.value = false }
  // Fetch active subscriptions (uses cache, non-blocking)
  subscriptionStore.fetchActiveSubscriptions().catch(() => {})
})
function bonusTierClass(idx: number, total: number): string {
  const rank = total <= 1 ? 0 : idx / (total - 1)
  if (rank >= 0.8) return 'bonus-tier-legendary'
  if (rank >= 0.5) return 'bonus-tier-epic'
  return 'bonus-tier-normal'
}

function bonusTierBadgeClass(idx: number, total: number): string {
  const rank = total <= 1 ? 0 : idx / (total - 1)
  if (rank >= 0.8) return 'bonus-badge-legendary'
  if (rank >= 0.5) return 'bonus-badge-epic'
  return 'bonus-badge-normal'
}
</script>

<style scoped>
.first-recharge-banner {
  position: relative;
  overflow: hidden;
  border-radius: 1rem;
  background: linear-gradient(135deg, #f59e0b, #ec4899, #8b5cf6, #3b82f6);
  background-size: 300% 300%;
  animation: gradient-shift 4s ease infinite;
}

.first-recharge-glow {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 30% 50%, rgba(255, 255, 255, 0.2) 0%, transparent 60%);
  animation: glow-pulse 3s ease-in-out infinite;
}

@keyframes gradient-shift {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

@keyframes glow-pulse {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

.bonus-tier-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.375rem;
  padding: 0.875rem 0.75rem;
  border-radius: 0.75rem;
  text-align: center;
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: pointer;
}
.bonus-tier-card:hover {
  transform: translateY(-1px);
}

.bonus-tier-selected {
  ring: 2px;
  outline: 2px solid #6366f1;
  outline-offset: 1px;
}

.bonus-tier-normal {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
}
:root.dark .bonus-tier-normal {
  background: rgba(31, 41, 55, 0.5);
  border-color: #374151;
}

.bonus-tier-epic {
  background: linear-gradient(135deg, #ede9fe, #dbeafe);
  border: 1px solid #c4b5fd;
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.1);
}
:root.dark .bonus-tier-epic {
  background: linear-gradient(135deg, rgba(139, 92, 246, 0.15), rgba(59, 130, 246, 0.15));
  border-color: rgba(139, 92, 246, 0.3);
}

.bonus-tier-legendary {
  background: linear-gradient(135deg, #fef3c7, #fce7f3, #ede9fe);
  border: 1px solid #fbbf24;
  box-shadow: 0 4px 15px rgba(251, 191, 36, 0.2), 0 0 0 1px rgba(251, 191, 36, 0.1);
  animation: legendary-glow 3s ease-in-out infinite;
}
:root.dark .bonus-tier-legendary {
  background: linear-gradient(135deg, rgba(251, 191, 36, 0.15), rgba(236, 72, 153, 0.12), rgba(139, 92, 246, 0.15));
  border-color: rgba(251, 191, 36, 0.4);
  box-shadow: 0 4px 15px rgba(251, 191, 36, 0.15);
}

@keyframes legendary-glow {
  0%, 100% { box-shadow: 0 4px 15px rgba(251, 191, 36, 0.2), 0 0 0 1px rgba(251, 191, 36, 0.1); }
  50% { box-shadow: 0 4px 20px rgba(251, 191, 36, 0.35), 0 0 0 2px rgba(251, 191, 36, 0.15); }
}

.bonus-tier-badge {
  font-size: 1.125rem;
  font-weight: 700;
  line-height: 1;
  padding: 0.25rem 0.625rem;
  border-radius: 9999px;
}

.bonus-badge-normal {
  color: #059669;
  background: #d1fae5;
}
:root.dark .bonus-badge-normal {
  color: #34d399;
  background: rgba(16, 185, 129, 0.15);
}

.bonus-badge-epic {
  color: #7c3aed;
  background: linear-gradient(135deg, #ede9fe, #ddd6fe);
}
:root.dark .bonus-badge-epic {
  color: #a78bfa;
  background: rgba(139, 92, 246, 0.2);
}

.bonus-badge-legendary {
  color: #b45309;
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  box-shadow: 0 0 8px rgba(251, 191, 36, 0.3);
}
:root.dark .bonus-badge-legendary {
  color: #fbbf24;
  background: linear-gradient(135deg, rgba(251, 191, 36, 0.25), rgba(245, 158, 11, 0.2));
  box-shadow: 0 0 8px rgba(251, 191, 36, 0.2);
}

.bonus-tier-threshold {
  font-size: 0.75rem;
  color: #6b7280;
  margin: 0;
}
:root.dark .bonus-tier-threshold {
  color: #9ca3af;
}
</style>
