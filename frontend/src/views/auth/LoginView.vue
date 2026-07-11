<template>
  <div class="login-page relative min-h-screen bg-white lg:flex">

    <!-- LEFT PANEL - Marketing hero (desktop only) -->
    <section class="relative hidden overflow-hidden bg-gradient-to-br from-[#2563EB] via-[#7C3AED] to-[#EC4899] lg:flex lg:w-1/2">
      <div class="pointer-events-none absolute inset-0 bg-black/20"></div>
      <!-- Decorative glow orbs -->
      <div class="pointer-events-none absolute inset-0 overflow-hidden">
        <div class="absolute -left-24 -top-24 h-[420px] w-[420px] rounded-full bg-white opacity-10 blur-3xl"></div>
        <div class="absolute -bottom-24 -right-16 h-[460px] w-[460px] rounded-full bg-white opacity-[0.09] blur-3xl"></div>
        <div class="absolute left-1/3 top-24 h-[260px] w-[260px] rounded-full bg-[#DBEAFE] opacity-10 blur-3xl"></div>
      </div>

      <div class="relative z-10 flex w-full flex-col px-12 py-12 xl:px-16">
        <!-- Brand -->
        <div v-if="settingsLoaded" class="flex items-center gap-4">
          <div class="flex h-12 w-12 items-center justify-center overflow-hidden rounded-2xl bg-white shadow-lg">
            <img v-if="siteLogo" :src="siteLogo" alt="Logo" class="h-8 w-8 object-contain" />
            <span v-else class="text-xl font-extrabold text-[#6D5DFC]">{{ brandInitial }}</span>
          </div>
          <span class="text-2xl font-bold text-white">{{ siteName }}</span>
        </div>

        <!-- Badge + Heading + Description -->
        <div class="mt-12 xl:mt-14">
          <span
            v-if="loginBadge"
            class="inline-block rounded-full bg-white/15 px-4 py-1.5 text-xs font-bold tracking-[0.2em] text-white/90"
          >{{ loginBadge }}</span>
          <h1 class="mt-5 text-[38px] font-bold leading-[1.22] text-white xl:text-[46px]">
            {{ loginHeading1 }}<br />{{ loginHeading2 }}
          </h1>
          <p class="mt-6 max-w-[560px] text-[15px] leading-7 text-white/85">{{ loginHeroDesc }}</p>
        </div>

        <!-- Live billing sample card -->
        <div class="mt-10 rounded-2xl border border-white/[0.22] bg-white/[0.12] p-6 shadow-[0_18px_36px_rgba(15,5,60,0.28)]">
          <div class="flex items-center justify-between">
            <span class="text-[13px] font-medium text-white/80">{{ t('auth.login.billCardTitle') }}</span>
            <span class="rounded-full bg-[#22C55E]/[0.22] px-3 py-1 text-[11px] font-bold text-[#BBF7D0]">
              {{ t('auth.login.billCardBadge') }}
            </span>
          </div>
          <div class="mt-4 space-y-3">
            <div v-for="row in billRows" :key="row.model" class="flex items-center justify-between gap-3">
              <span class="shrink-0 text-sm font-bold text-white">{{ row.model }}</span>
              <span class="flex-1 truncate pl-2 text-xs text-white/60">{{ row.tokens }}</span>
              <span class="shrink-0 text-sm font-bold text-white">{{ row.price }}</span>
            </div>
          </div>
          <div class="my-4 h-px bg-white/[0.16]"></div>
          <div class="flex items-center justify-between">
            <span class="text-xs text-white/60">{{ t('auth.login.billCardTotalLabel') }}</span>
            <span class="text-base font-bold text-white">{{ t('auth.login.billCardTotal') }}</span>
          </div>
        </div>

        <!-- Model cards -->
        <div class="mt-8 grid grid-cols-3 gap-5">
          <div
            v-for="card in modelCards"
            :key="card.name"
            class="rounded-[14px] border border-white/[0.28] bg-white/[0.17] p-4 shadow-[0_14px_36px_rgba(15,5,60,0.18)]"
          >
            <div class="truncate text-lg font-bold text-white xl:text-xl">{{ card.name }}</div>
            <div class="mt-1.5 truncate text-xs font-medium text-white/[0.78]">{{ card.desc }}</div>
          </div>
        </div>

        <!-- Stats -->
        <div class="mt-auto grid grid-cols-3 gap-5 pt-9">
          <div v-for="stat in statItems" :key="stat.label">
            <div class="text-3xl font-bold text-white xl:text-[34px] xl:leading-none">{{ stat.value }}</div>
            <div class="mt-2 text-sm text-[#F4EAFF]/[0.85]">{{ stat.label }}</div>
          </div>
        </div>
      </div>
    </section>

    <!-- RIGHT PANEL - Login form -->
    <section class="relative flex min-h-screen w-full flex-col lg:w-1/2">
      <!-- Top navigation (desktop) -->
      <nav class="absolute right-8 top-6 z-20 hidden items-center gap-3 lg:flex">
        <router-link to="/key-usage" class="login-nav-pill">
          {{ t('auth.login.navKeyUsage') }}
        </router-link>
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener"
          class="login-nav-pill"
        >
          {{ t('auth.login.navDocs') }}
        </a>
      </nav>

      <!-- Mobile hero (small screens only) -->
      <div class="relative overflow-hidden bg-gradient-to-br from-[#2563EB] via-[#7C3AED] to-[#EC4899] px-6 pb-16 pt-10 lg:hidden">
        <div class="pointer-events-none absolute inset-0 bg-black/[0.18]"></div>
        <div class="relative z-10">
          <div v-if="settingsLoaded" class="flex items-center gap-3">
            <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-lg bg-white">
              <img v-if="siteLogo" :src="siteLogo" alt="Logo" class="h-5 w-5 object-contain" />
              <span v-else class="text-sm font-extrabold text-[#6D5DFC]">{{ brandInitial }}</span>
            </div>
            <span class="text-lg font-bold text-white">{{ siteName }}</span>
          </div>
          <h1 class="mt-7 text-[28px] font-bold leading-[1.25] text-white">
            {{ loginHeading1 }}<br />{{ loginHeading2 }}
          </h1>
          <p class="mt-4 text-[15px] font-bold text-[#F5EFFF]">{{ t('auth.login.mobileHeroModels') }}</p>
          <p class="mt-2 text-[13px] leading-5 text-[#F4EAFF]/[0.92]">{{ t('auth.login.mobileHeroDesc') }}</p>
        </div>
      </div>

      <!-- Form column -->
      <div class="relative z-10 mx-auto flex w-full max-w-[456px] flex-1 flex-col px-4 pb-10 sm:px-6 lg:justify-center lg:px-8 lg:py-20">
        <!-- On mobile this is a floating card overlapping the hero; on desktop it sits flat -->
        <div class="-mt-9 rounded-2xl border border-gray-100 bg-white p-6 shadow-[0_24px_48px_rgba(15,23,42,0.16)] sm:p-7 lg:mt-0 lg:rounded-none lg:border-0 lg:p-0 lg:shadow-none">
          <!-- Title -->
          <h2 class="text-[26px] font-bold text-gray-900 lg:text-[32px]">{{ loginFormTitle }}</h2>
          <p class="mt-2 text-sm text-gray-500">{{ loginFormSubtitle }}</p>

          <!-- Trust badges (desktop) -->
          <div class="mt-5 hidden flex-wrap gap-2.5 lg:flex">
            <span
              v-for="badge in trustBadges"
              :key="badge"
              class="inline-flex items-center gap-2 rounded-full bg-[#F4F2FF] px-3.5 py-2 text-[13px] font-medium text-gray-600"
            >
              <span class="h-1.5 w-1.5 rounded-full bg-[#6D5DFC]"></span>
              {{ badge }}
            </span>
          </div>

          <!-- Login Form -->
          <form class="mt-7" @submit.prevent="handleLogin">
            <!-- Email -->
            <div class="mb-5">
              <label for="email" class="mb-2 block text-[13px] font-semibold text-gray-700">
                {{ t('auth.emailLabel') }}
              </label>
              <div class="relative">
                <Icon
                  name="mail"
                  size="md"
                  class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400"
                />
                <input
                  id="email"
                  v-model="formData.email"
                  type="email"
                  required
                  autofocus
                  autocomplete="email"
                  :disabled="isLoading"
                  :placeholder="t('auth.emailPlaceholder')"
                  class="login-input"
                  :class="{ 'login-input-error': errors.email }"
                />
              </div>
              <p v-if="errors.email" class="mt-1.5 text-xs text-red-500">
                {{ errors.email }}
              </p>
            </div>

            <!-- Password -->
            <div class="mb-5">
              <label for="password" class="mb-2 block text-[13px] font-semibold text-gray-700">
                {{ t('auth.passwordLabel') }}
              </label>
              <div class="relative">
                <Icon
                  name="lock"
                  size="md"
                  class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400"
                />
                <input
                  id="password"
                  v-model="formData.password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  autocomplete="current-password"
                  :disabled="isLoading"
                  :placeholder="t('auth.passwordPlaceholder')"
                  class="login-input pr-11"
                  :class="{ 'login-input-error': errors.password }"
                />
                <button
                  type="button"
                  class="absolute inset-y-0 right-0 flex items-center pr-4 text-gray-400 transition-colors hover:text-gray-600"
                  @click="showPassword = !showPassword"
                >
                  <Icon v-if="showPassword" name="eyeOff" size="md" />
                  <Icon v-else name="eye" size="md" />
                </button>
              </div>
              <div class="mt-1.5 flex items-center justify-between">
                <p v-if="errors.password" class="text-xs text-red-500">
                  {{ errors.password }}
                </p>
                <span v-else></span>
                <router-link
                  v-if="passwordResetEnabled && !backendModeEnabled"
                  to="/forgot-password"
                  class="text-[13px] font-medium text-[#6D5DFC] transition-colors hover:text-[#8B7BFF]"
                >
                  {{ t('auth.forgotPassword') }}
                </router-link>
              </div>
            </div>

            <!-- Turnstile Widget -->
            <div v-if="turnstileEnabled && turnstileSiteKey" class="mb-5">
              <TurnstileWidget
                ref="turnstileRef"
                :site-key="turnstileSiteKey"
                theme="light"
                @verify="onTurnstileVerify"
                @expire="onTurnstileExpire"
                @error="onTurnstileError"
              />
              <p v-if="errors.turnstile" class="mt-2 text-center text-xs text-red-500">
                {{ errors.turnstile }}
              </p>
            </div>

            <!-- Error Message -->
            <transition name="fade">
              <div
                v-if="errorMessage"
                class="mb-5 rounded-xl border border-red-200 bg-red-50 p-3"
              >
                <div class="flex items-start gap-2">
                  <Icon name="exclamationCircle" size="md" class="mt-0.5 flex-shrink-0 text-red-500" />
                  <p class="text-sm text-red-600">{{ errorMessage }}</p>
                </div>
              </div>
            </transition>

            <!-- Submit Button -->
            <button
              type="submit"
              :disabled="isLoading || (turnstileEnabled && !turnstileToken)"
              class="login-btn w-full"
            >
              <svg
                v-if="isLoading"
                class="-ml-1 mr-2 h-4 w-4 animate-spin text-white"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ isLoading ? t('auth.signingIn') : t('auth.login.submitButton') }}
            </button>

            <!-- Register Button -->
            <router-link
              v-if="!backendModeEnabled"
              to="/register"
              class="mt-3 flex w-full items-center justify-center rounded-xl border border-gray-300 bg-white py-3.5 text-sm font-bold text-gray-700 transition-colors hover:bg-gray-50"
            >
              {{ t('auth.login.registerButton') }}
            </router-link>
          </form>

          <!-- OAuth Section -->
          <div v-if="!backendModeEnabled && (linuxdoOAuthEnabled || wechatOAuthEnabled || oidcOAuthEnabled)" class="mt-6">
            <!-- Divider -->
            <div class="flex items-center gap-3">
              <div class="h-px flex-1 bg-gray-200"></div>
              <span class="text-[13px] text-gray-400">{{ t('auth.login.socialDivider') }}</span>
              <div class="h-px flex-1 bg-gray-200"></div>
            </div>

            <!-- OAuth Buttons -->
            <div class="login-oauth mt-4 flex gap-3">
              <LinuxDoOAuthSection
                v-if="linuxdoOAuthEnabled"
                :disabled="isLoading"
                :show-divider="false"
                class="flex-1"
              />
              <WechatOAuthSection
                v-if="wechatOAuthEnabled"
                :disabled="isLoading"
                :show-divider="false"
                class="flex-1"
              />
              <OidcOAuthSection
                v-if="oidcOAuthEnabled"
                :disabled="isLoading"
                :provider-name="oidcOAuthProviderName"
                :show-divider="false"
                class="flex-1"
              />
            </div>
          </div>

          <!-- Trust chips (mobile) -->
          <div class="mt-6 flex flex-wrap justify-center gap-2 lg:hidden">
            <span
              v-for="badge in trustBadges"
              :key="badge"
              class="rounded-full bg-[#F4F2FF] px-3 py-1.5 text-[11px] font-medium text-[#6D5DFC]"
            >
              {{ badge }}
            </span>
          </div>

          <!-- Nav links (mobile) -->
          <div class="mt-5 flex items-center justify-center gap-5 lg:hidden">
            <router-link to="/key-usage" class="text-[13px] font-medium text-gray-500 transition-colors hover:text-gray-700">
              {{ t('auth.login.navKeyUsage') }}
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener"
              class="text-[13px] font-medium text-gray-500 transition-colors hover:text-gray-700"
            >
              {{ t('auth.login.navDocs') }}
            </a>
          </div>
        </div>

        <!-- Capability cards (desktop) -->
        <div class="mt-10 hidden grid-cols-2 gap-4 lg:grid">
          <div
            v-for="cap in capabilityCards"
            :key="cap.title"
            class="rounded-xl border border-[#EEF0F5] bg-[#F8F9FC] p-4"
          >
            <div class="text-sm font-bold text-gray-900">{{ cap.title }}</div>
            <div class="mt-1.5 text-xs leading-5 text-gray-500">{{ cap.desc }}</div>
          </div>
        </div>
      </div>
    </section>

    <!-- 2FA Modal -->
    <TotpLoginModal
      v-if="show2FAModal"
      ref="totpModalRef"
      :temp-token="totpTempToken"
      :user-email-masked="totpUserEmailMasked"
      @verify="handle2FAVerify"
      @cancel="handle2FACancel"
    />

    <LegalConsentDialog
      :show="showLegalConsentDialog"
      mode="login"
      :settings="legalConsentSettings"
      @accept="handleLegalConsentAccepted"
      @cancel="handleLegalConsentCancelled"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LinuxDoOAuthSection from '@/components/auth/LinuxDoOAuthSection.vue'
import OidcOAuthSection from '@/components/auth/OidcOAuthSection.vue'
import WechatOAuthSection from '@/components/auth/WechatOAuthSection.vue'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import LegalConsentDialog from '@/components/auth/LegalConsentDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicSettings, isTotp2FARequired, isWeChatWebOAuthEnabled } from '@/api/auth'
import { sanitizeUrl } from '@/utils/url'
import { buildAuthErrorMessage } from '@/utils/authError'
import type { LegalConsentSettings, TotpLoginResponse, User } from '@/types'
import { clearAllAffiliateReferralCodes } from '@/utils/oauthAffiliate'
import {
  hasAcceptedCurrentLegalConsent,
  markLegalConsentAccepted,
  resolveLegalConsentSettings,
  type LegalConsentPayload
} from '@/utils/legalConsent'

const { t } = useI18n()

// ==================== Router & Stores ====================

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// ==================== Site Info (replaces AuthLayout) ====================

const siteName = computed(() => appStore.siteName || 'ZeroCode')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const docUrl = computed(() => sanitizeUrl(appStore.docUrl || ''))
const brandInitial = computed(() => (siteName.value || 'Z').trim().charAt(0).toUpperCase())

// ==================== Login Page Content Overrides ====================
// Admin-editable copy from settings (public). Empty / missing fields fall back
// to the i18n defaults below, so deleting a field in admin = restoring the
// original translation. Keeps every language correct without per-field branching.
const loginPageOverrides = computed(() => appStore.cachedPublicSettings?.login_page ?? null)
const localLegalConsentSettings = ref<LegalConsentSettings | null>(null)
const legalConsentSettings = computed(() => resolveLegalConsentSettings(
  localLegalConsentSettings.value || appStore.cachedPublicSettings?.legal_consent
))
const pickLoginText = (value: string | undefined | null, fallback: string): string => {
  const v = typeof value === 'string' ? value.trim() : ''
  return v !== '' ? v : fallback
}
const loginBadge = computed(() => pickLoginText(loginPageOverrides.value?.badge, t('auth.login.badge')))
const loginHeading1 = computed(() => pickLoginText(loginPageOverrides.value?.heading_line1, t('auth.login.headingLine1')))
const loginHeading2 = computed(() => pickLoginText(loginPageOverrides.value?.heading_line2, t('auth.login.headingLine2')))
const loginHeroDesc = computed(() => pickLoginText(loginPageOverrides.value?.description, t('auth.login.description')))
const loginFormTitle = computed(() => pickLoginText(loginPageOverrides.value?.form_title, t('auth.login.title')))
const loginFormSubtitle = computed(() => pickLoginText(loginPageOverrides.value?.form_subtitle, t('auth.login.subtitle')))

// ==================== Marketing Content (i18n-driven) ====================
// 左侧账单示例卡 / 模型卡 / 统计与右侧信任徽章、能力卡的文案都来自 i18n，
// 中英文各配一套（`auth.login.*`），改文案只动 locale 文件即可。

const billRows = computed(() => [
  { model: t('auth.login.billRow1Model'), tokens: t('auth.login.billRow1Tokens'), price: t('auth.login.billRow1Price') },
  { model: t('auth.login.billRow2Model'), tokens: t('auth.login.billRow2Tokens'), price: t('auth.login.billRow2Price') }
])

const modelCards = computed(() => [
  { name: t('auth.login.modelCard1Name'), desc: t('auth.login.modelCard1Desc') },
  { name: t('auth.login.modelCard2Name'), desc: t('auth.login.modelCard2Desc') },
  { name: t('auth.login.modelCard3Name'), desc: t('auth.login.modelCard3Desc') }
])

const statItems = computed(() => [
  { value: t('auth.login.stat1Value'), label: t('auth.login.stat1Label') },
  { value: t('auth.login.stat2Value'), label: t('auth.login.stat2Label') },
  { value: t('auth.login.stat3Value'), label: t('auth.login.stat3Label') }
])

const trustBadges = computed(() => [
  t('auth.login.trustBadge1'),
  t('auth.login.trustBadge2'),
  t('auth.login.trustBadge3')
])

const capabilityCards = computed(() => [
  { title: t('auth.login.capImageTitle'), desc: t('auth.login.capImageDesc') },
  { title: t('auth.login.capTutorialTitle'), desc: t('auth.login.capTutorialDesc') }
])

// ==================== State ====================

const isLoading = ref<boolean>(false)
const errorMessage = ref<string>('')
const showPassword = ref<boolean>(false)

// Public settings
const turnstileEnabled = ref<boolean>(false)
const turnstileSiteKey = ref<string>('')
const linuxdoOAuthEnabled = ref<boolean>(false)
const wechatOAuthEnabled = ref<boolean>(false)
const backendModeEnabled = ref<boolean>(false)
const oidcOAuthEnabled = ref<boolean>(false)
const oidcOAuthProviderName = ref<string>('OIDC')
const passwordResetEnabled = ref<boolean>(false)

// Turnstile
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const turnstileToken = ref<string>('')

// 2FA state
const show2FAModal = ref<boolean>(false)
const totpTempToken = ref<string>('')
const totpUserEmailMasked = ref<string>('')
const totpModalRef = ref<InstanceType<typeof TotpLoginModal> | null>(null)
const showLegalConsentDialog = ref<boolean>(false)
const pendingRedirectAfterConsent = ref<string>('/dashboard')

const formData = reactive({
  email: '',
  password: ''
})

const errors = reactive({
  email: '',
  password: '',
  turnstile: ''
})

const validationToastMessage = computed(
  () => errors.email || errors.password || errors.turnstile || ''
)

watch(validationToastMessage, (value, previousValue) => {
  if (value && value !== previousValue) {
    appStore.showError(value)
  }
})

// ==================== Lifecycle ====================

onMounted(async () => {
  appStore.fetchPublicSettings()

  const expiredFlag = sessionStorage.getItem('auth_expired')
  if (expiredFlag) {
    sessionStorage.removeItem('auth_expired')
    const message = t('auth.reloginRequired')
    errorMessage.value = message
    appStore.showWarning(message)
  }

  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled
    wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
    backendModeEnabled.value = settings.backend_mode_enabled
    oidcOAuthEnabled.value = settings.oidc_oauth_enabled
    oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
    backendModeEnabled.value = settings.backend_mode_enabled
    passwordResetEnabled.value = settings.password_reset_enabled
    localLegalConsentSettings.value = settings.legal_consent || null
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
})

// ==================== Turnstile Handlers ====================

function onTurnstileVerify(token: string): void {
  turnstileToken.value = token
  errors.turnstile = ''
}

function onTurnstileExpire(): void {
  turnstileToken.value = ''
  errors.turnstile = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  turnstileToken.value = ''
  errors.turnstile = t('auth.turnstileFailed')
}

// ==================== Validation ====================

function validateForm(): boolean {
  // Reset errors
  errors.email = ''
  errors.password = ''
  errors.turnstile = ''

  let isValid = true

  // Email validation
  if (!formData.email.trim()) {
    errors.email = t('auth.emailRequired')
    isValid = false
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
    errors.email = t('auth.invalidEmail')
    isValid = false
  }

  // Password validation
  if (!formData.password) {
    errors.password = t('auth.passwordRequired')
    isValid = false
  } else if (formData.password.length < 6) {
    errors.password = t('auth.passwordMinLength')
    isValid = false
  }

  // Turnstile validation
  if (turnstileEnabled.value && !turnstileToken.value) {
    errors.turnstile = t('auth.completeVerification')
    isValid = false
  }

  return isValid
}

// ==================== Form Handlers ====================

async function handleLogin(): Promise<void> {
  // Clear previous error
  errorMessage.value = ''

  // Validate form
  if (!validateForm()) {
    return
  }

  isLoading.value = true

  try {
    // Call auth store login
    const response = await authStore.login({
      email: formData.email,
      password: formData.password,
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined
    })

    // Check if 2FA is required
    if (isTotp2FARequired(response)) {
      const totpResponse = response as TotpLoginResponse
      totpTempToken.value = totpResponse.temp_token || ''
      totpUserEmailMasked.value = totpResponse.user_email_masked || ''
      show2FAModal.value = true
      isLoading.value = false
      return
    }

    // Redirect to dashboard or intended route
    const redirectTo = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    await completeAuthenticatedLogin(response.user, redirectTo)
  } catch (error: unknown) {
    // Reset Turnstile on error
    if (turnstileRef.value) {
      turnstileRef.value.reset()
      turnstileToken.value = ''
    }

    errorMessage.value = buildAuthErrorMessage(error, {
      fallback: t('auth.loginFailed'),
      pendingApproval: t('auth.pendingApprovalLoginBlocked')
    })

    // Also show error toast
    appStore.showError(errorMessage.value)
  } finally {
    isLoading.value = false
  }
}

// ==================== 2FA Handlers ====================

async function handle2FAVerify(code: string): Promise<void> {
  if (totpModalRef.value) {
    totpModalRef.value.setVerifying(true)
  }

  try {
    await authStore.login2FA(totpTempToken.value, code)

    show2FAModal.value = false
    const redirectTo = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    await completeAuthenticatedLogin(authStore.user as User | null, redirectTo)
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { message?: string } } }
    const message = err.response?.data?.message || err.message || t('profile.totp.loginFailed')

    if (totpModalRef.value) {
      totpModalRef.value.setError(message)
      totpModalRef.value.setVerifying(false)
    }
  }
}

function handle2FACancel(): void {
  show2FAModal.value = false
  totpTempToken.value = ''
  totpUserEmailMasked.value = ''
}

async function completeAuthenticatedLogin(user: User | null | undefined, redirectTo: string): Promise<void> {
  pendingRedirectAfterConsent.value = redirectTo || '/dashboard'
  if (!appStore.publicSettingsLoaded) {
    const settings = await appStore.fetchPublicSettings()
    localLegalConsentSettings.value = settings?.legal_consent || localLegalConsentSettings.value
  }

  if (!user?.id || !hasAcceptedCurrentLegalConsent(user.id, legalConsentSettings.value)) {
    showLegalConsentDialog.value = true
    return
  }

  await finishLoginRedirect()
}

async function handleLegalConsentAccepted(payload: LegalConsentPayload): Promise<void> {
  if (authStore.user?.id) {
    markLegalConsentAccepted(authStore.user.id, {
      ...payload,
      source: 'login'
    }, legalConsentSettings.value)
  }
  showLegalConsentDialog.value = false
  await finishLoginRedirect()
}

async function finishLoginRedirect(): Promise<void> {
  clearAllAffiliateReferralCodes()
  appStore.showSuccess(t('auth.loginSuccess'))
  await router.push(pendingRedirectAfterConsent.value || '/dashboard')
}

async function handleLegalConsentCancelled(): Promise<void> {
  showLegalConsentDialog.value = false
  await authStore.logout().catch(() => undefined)
}
</script>

<style scoped>
/* ============ Login Page Input (light theme) ============ */
.login-input {
  @apply w-full rounded-xl border bg-white py-3.5 pl-11 pr-4 text-sm text-gray-900 transition-all duration-200;
  border-color: #d1d5db;
}

.login-input::placeholder {
  color: #9ca3af;
}

.login-input:focus {
  outline: none;
  border-color: #6d5dfc;
  box-shadow: 0 0 0 3px rgba(109, 93, 252, 0.14);
}

.login-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.login-input-error {
  border-color: rgba(239, 68, 68, 0.6) !important;
}

/* ============ Login Button (brand gradient) ============ */
.login-btn {
  @apply flex items-center justify-center rounded-xl py-3.5 text-[15px] font-bold text-white transition-all duration-200;
  background: linear-gradient(to right, #6d5dfc, #a855f7);
  box-shadow: 0 10px 24px rgba(109, 93, 252, 0.35);
}

.login-btn:hover:not(:disabled) {
  opacity: 0.92;
}

.login-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.login-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  box-shadow: none;
}

/* ============ Top Nav Pills ============ */
.login-nav-pill {
  @apply rounded-full bg-gray-100 px-4 py-2 text-[13px] font-semibold text-gray-600 transition-colors hover:bg-gray-200;
}

/* ============ OAuth Button Override (light theme) ============ */
.login-oauth :deep(.btn.btn-secondary) {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  color: #374151;
  border-radius: 12px;
  padding: 12px 16px;
  font-weight: 700;
  font-size: 14px;
}

.login-oauth :deep(.btn.btn-secondary:hover) {
  background: #f9fafb;
  border-color: #d1d5db;
}

.login-oauth :deep(.btn.btn-secondary) .icon {
  width: 20px;
  height: 20px;
}

/* ============ Transitions ============ */
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
