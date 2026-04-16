<template>
  <div class="login-page relative min-h-screen overflow-hidden bg-gradient-to-br from-[#07111B] via-[#0B1623] to-[#102335]">
    <!-- Decorative Glow Orbs -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute -left-20 -top-20 h-[440px] w-[440px] rounded-full bg-[#18D8AA] opacity-[0.08] blur-3xl"></div>
      <div class="absolute -top-20 right-20 h-[428px] w-[428px] rounded-full bg-[#49A9FF] opacity-[0.10] blur-3xl"></div>
      <div class="absolute -bottom-20 right-40 h-[496px] w-[496px] rounded-full bg-[#19D7A9] opacity-[0.05] blur-3xl"></div>
    </div>

    <!-- Main Container -->
    <div class="relative z-10 mx-auto flex min-h-screen max-w-[1520px] flex-col px-5 py-6 lg:px-10 lg:py-8">

      <!-- Outer Card Border -->
      <div class="flex flex-1 flex-col rounded-[36px] border border-white/[0.08] bg-white/[0.03] p-4 lg:p-6">

        <!-- Top Navigation Bar -->
        <nav class="mb-4 flex items-center justify-between lg:mb-6">
          <!-- Left: spacer -->
          <div></div>

          <!-- Right: Nav Pills -->
          <div class="hidden items-center gap-3 sm:flex">
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener"
              class="rounded-[17px] bg-white/[0.06] px-5 py-2 text-[13px] font-semibold text-[#DCE7F2] transition-colors hover:bg-white/[0.10]"
            >
              {{ t('auth.login.navDocs') }}
            </a>
          </div>
        </nav>

        <!-- Main Content Area -->
        <div class="flex flex-1 flex-col rounded-[38px] border border-white/[0.08] bg-white/[0.03] lg:flex-row">

          <!-- LEFT PANEL - Marketing (hidden on mobile) -->
          <div class="hidden flex-1 flex-col justify-between rounded-l-[32px] bg-gradient-to-br from-[#18D8AA]/[0.16] to-[#4BA8FF]/[0.04] border border-white/[0.08] p-8 lg:flex xl:p-12">
            <!-- Badge -->
            <div>
              <span class="inline-block rounded-[20px] bg-[#ECFFF9] px-4 py-2 text-[13px] font-extrabold tracking-wide text-[#0D2A3C]">
                {{ t('auth.login.badge') }}
              </span>

              <!-- Heading -->
              <h1 class="mt-12 text-[48px] font-extrabold leading-[1.15] text-white xl:text-[56px]">
                {{ t('auth.login.headingLine1') }}
              </h1>
              <h1 class="text-[48px] font-extrabold leading-[1.15] text-[#9BFFEA] xl:text-[56px]">
                {{ t('auth.login.headingLine2') }}
              </h1>
              <p class="mt-4 max-w-[540px] text-base leading-relaxed text-[#A9BDCF]">
                {{ t('auth.login.description') }}
              </p>

              <!-- Feature Pills -->
              <div class="mt-6 flex flex-wrap gap-3">
                <span class="rounded-[27px] bg-[#ECFFF9] px-5 py-3 text-lg font-extrabold text-[#0F2638]">
                  {{ t('auth.login.featurePrice') }}
                </span>
                <span class="rounded-[27px] bg-white/[0.08] px-5 py-3 text-base font-bold text-[#F1F5F9]">
                  {{ t('auth.login.featurePayAsYouGo') }}
                </span>
                <span class="rounded-[27px] bg-white/[0.08] px-5 py-3 text-base font-bold text-[#F1F5F9]">
                  {{ t('auth.login.featureNoCharge') }}
                </span>
              </div>
            </div>

            <!-- Supported Models -->
            <div class="-mt-[60px]">
              <p class="mb-3 text-base font-bold text-[#EAF6FF]">{{ t('auth.login.supportedModels') }}</p>
              <div class="flex flex-wrap gap-3">
                <div
                  v-for="(model, index) in modelCards"
                  :key="model"
                  :class="index === 0
                    ? 'rounded-[22px] bg-[#ECFFF9] px-5 py-4 text-lg font-extrabold text-[#0F2638]'
                    : 'rounded-[22px] border border-[#29475F] bg-[#102233] px-5 py-4 text-lg font-extrabold text-[#F4FAFF]'"
                >
                  {{ model }}
                </div>
              </div>

              <!-- Feature Cards -->
              <div class="mt-5 flex flex-wrap gap-3">
                <div class="flex-1 min-w-[160px] rounded-[24px] border border-[#29465D] bg-[#102233] px-5 py-4">
                  <div class="text-lg font-extrabold text-[#EAF5FF]">{{ t('auth.login.featureUnifiedApi') }}</div>
                  <div class="mt-1 text-xs text-[#9FB4C8]">{{ t('auth.login.featureUnifiedApiDesc') }}</div>
                </div>
                <div class="flex-1 min-w-[160px] rounded-[24px] border border-[#29465D] bg-[#102233] px-5 py-4">
                  <div class="text-lg font-extrabold text-[#EAF5FF]">{{ t('auth.login.featureTransparentBilling') }}</div>
                  <div class="mt-1 text-xs text-[#9FB4C8]">{{ t('auth.login.featureTransparentBillingDesc') }}</div>
                </div>
                <div class="flex-1 min-w-[160px] rounded-[24px] border border-[#29465D] bg-[#102233] px-5 py-4">
                  <div class="text-lg font-extrabold text-[#EAF5FF]">{{ t('auth.login.featureAuditTrail') }}</div>
                  <div class="mt-1 text-xs text-[#9FB4C8]">{{ t('auth.login.featureAuditTrailDesc') }}</div>
                </div>
              </div>
            </div>
          </div>

          <!-- RIGHT PANEL - Login Form -->
          <div class="flex w-full items-center justify-center p-6 lg:w-[420px] lg:p-8 xl:w-[480px] xl:p-10">
            <div class="w-full max-w-[414px]">

              <!-- Mobile brand (shown only on small screens) -->
              <div v-if="settingsLoaded" class="mb-6 text-center lg:hidden">
                <div class="mb-2 inline-flex h-12 w-12 items-center justify-center overflow-hidden rounded-xl border border-[#25435C] bg-[#0C1A29]">
                  <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-8 w-8 object-contain" />
                </div>
                <div class="text-lg font-bold text-[#EFF6FF]">{{ siteName }}</div>
              </div>

              <!-- Title -->
              <h2 class="text-[28px] font-extrabold text-[#F6FBFF]">{{ t('auth.login.title') }}</h2>
              <p class="mt-2 text-[14px] text-[#8EA6BD]">{{ t('auth.login.subtitle') }}</p>

              <!-- Login Form -->
              <form class="mt-8" @submit.prevent="handleLogin">
                <!-- Email -->
                <div class="mb-5">
                  <label for="email" class="mb-2 block text-[13px] font-bold text-[#C8D7E4]">
                    {{ t('auth.emailLabel') }}
                  </label>
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
                  <p v-if="errors.email" class="mt-1.5 text-xs text-red-400">
                    {{ errors.email }}
                  </p>
                </div>

                <!-- Password -->
                <div class="mb-5">
                  <label for="password" class="mb-2 block text-[13px] font-bold text-[#C8D7E4]">
                    {{ t('auth.passwordLabel') }}
                  </label>
                  <div class="relative">
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
                      class="absolute inset-y-0 right-0 flex items-center pr-4 text-[#7F97AE] transition-colors hover:text-[#C8D7E4]"
                      @click="showPassword = !showPassword"
                    >
                      <Icon v-if="showPassword" name="eyeOff" size="md" />
                      <Icon v-else name="eye" size="md" />
                    </button>
                  </div>
                  <div class="mt-1.5 flex items-center justify-between">
                    <p v-if="errors.password" class="text-xs text-red-400">
                      {{ errors.password }}
                    </p>
                    <span v-else></span>
                    <router-link
                      v-if="passwordResetEnabled && !backendModeEnabled"
                      to="/forgot-password"
                      class="text-xs font-medium text-[#7FE9D4] transition-colors hover:text-[#9BFFEA]"
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
                    theme="dark"
                    @verify="onTurnstileVerify"
                    @expire="onTurnstileExpire"
                    @error="onTurnstileError"
                  />
                  <p v-if="errors.turnstile" class="mt-2 text-center text-xs text-red-400">
                    {{ errors.turnstile }}
                  </p>
                </div>

                <!-- Error Message -->
                <transition name="fade">
                  <div
                    v-if="errorMessage"
                    class="mb-5 rounded-xl border border-red-500/30 bg-red-500/10 p-3"
                  >
                    <div class="flex items-start gap-2">
                      <Icon name="exclamationCircle" size="md" class="mt-0.5 flex-shrink-0 text-red-400" />
                      <p class="text-sm text-red-300">{{ errorMessage }}</p>
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
              </form>

              <!-- Post-login Info -->
              <div class="mt-6">
                <p class="text-[13px] font-bold text-[#7FE9D4]">{{ t('auth.login.postLoginInfo') }}</p>
                <p class="mt-1 text-sm font-bold text-[#F4FAFF]">{{ t('auth.login.postLoginDetails') }}</p>
              </div>

              <!-- OAuth Section -->
              <div v-if="!backendModeEnabled && (linuxdoOAuthEnabled || oidcOAuthEnabled)" class="mt-6">
                <!-- Divider -->
                <div class="flex items-center gap-3">
                  <div class="h-px flex-1 bg-white/[0.12]"></div>
                  <span class="text-[13px] text-[#7F97AE]">{{ t('auth.login.socialDivider') }}</span>
                  <div class="h-px flex-1 bg-white/[0.12]"></div>
                </div>

                <!-- OAuth Buttons -->
                <div class="login-oauth mt-4 flex gap-3">
                  <LinuxDoOAuthSection
                    v-if="linuxdoOAuthEnabled"
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

              <!-- Register Link -->
              <div v-if="!backendModeEnabled" class="mt-6 text-center">
                <p class="text-sm text-[#7F97AE]">
                  {{ t('auth.dontHaveAccount') }}
                  <router-link to="/register" class="font-medium text-[#7FE9D4] transition-colors hover:text-[#9BFFEA]">
                    {{ t('auth.signUp') }}
                  </router-link>
                </p>
              </div>

            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 2FA Modal -->
    <TotpLoginModal
      v-if="show2FAModal"
      ref="totpModalRef"
      :temp-token="totpTempToken"
      :user-email-masked="totpUserEmailMasked"
      @verify="handle2FAVerify"
      @cancel="handle2FACancel"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LinuxDoOAuthSection from '@/components/auth/LinuxDoOAuthSection.vue'
import OidcOAuthSection from '@/components/auth/OidcOAuthSection.vue'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicSettings, isTotp2FARequired } from '@/api/auth'
import { sanitizeUrl } from '@/utils/url'
import type { TotpLoginResponse } from '@/types'

const { t } = useI18n()

// ==================== Router & Stores ====================

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// ==================== Site Info (replaces AuthLayout) ====================

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const docUrl = computed(() => appStore.docUrl)

// ==================== Model Cards ====================

const modelCards = [
  'Claude 4.6',
  'Gemini 3.1',
  'GPT 5.4',
]

// ==================== State ====================

const isLoading = ref<boolean>(false)
const errorMessage = ref<string>('')
const showPassword = ref<boolean>(false)

// Public settings
const turnstileEnabled = ref<boolean>(false)
const turnstileSiteKey = ref<string>('')
const linuxdoOAuthEnabled = ref<boolean>(false)
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

const formData = reactive({
  email: '',
  password: ''
})

const errors = reactive({
  email: '',
  password: '',
  turnstile: ''
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
    backendModeEnabled.value = settings.backend_mode_enabled
    oidcOAuthEnabled.value = settings.oidc_oauth_enabled
    oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
    backendModeEnabled.value = settings.backend_mode_enabled
    passwordResetEnabled.value = settings.password_reset_enabled
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

    // Show success toast
    appStore.showSuccess(t('auth.loginSuccess'))

    // Redirect to dashboard or intended route
    const redirectTo = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    await router.push(redirectTo)
  } catch (error: unknown) {
    // Reset Turnstile on error
    if (turnstileRef.value) {
      turnstileRef.value.reset()
      turnstileToken.value = ''
    }

    // Handle login error
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    if (err.response?.data?.detail) {
      errorMessage.value = err.response.data.detail
    } else if (err.message) {
      errorMessage.value = err.message
    } else {
      errorMessage.value = t('auth.loginFailed')
    }

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

    // Close modal and show success
    show2FAModal.value = false
    appStore.showSuccess(t('auth.loginSuccess'))

    // Redirect to dashboard or intended route
    const redirectTo = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    await router.push(redirectTo)
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
</script>

<style scoped>
/* ============ Login Page Input ============ */
.login-input {
  @apply w-full rounded-[20px] border px-4 py-4 text-sm transition-all duration-200;
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.10);
  color: #EFF6FF;
}

.login-input::placeholder {
  color: #7F97AE;
}

.login-input:focus {
  outline: none;
  border-color: rgba(20, 213, 165, 0.4);
  box-shadow: 0 0 0 3px rgba(20, 213, 165, 0.12);
}

.login-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.login-input-error {
  border-color: rgba(239, 68, 68, 0.5) !important;
}

/* ============ Login Button ============ */
.login-btn {
  @apply flex items-center justify-center rounded-[20px] py-4 text-base font-extrabold text-white transition-all duration-200;
  background: linear-gradient(to right, #14D5A5, #239FFF);
}

.login-btn:hover:not(:disabled) {
  opacity: 0.9;
  box-shadow: 0 8px 24px rgba(20, 213, 165, 0.25);
}

.login-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.login-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* ============ OAuth Button Override ============ */
.login-oauth :deep(.btn.btn-secondary) {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.10);
  color: #F4FAFF;
  border-radius: 18px;
  padding: 14px 16px;
  font-weight: 700;
  font-size: 14px;
}

.login-oauth :deep(.btn.btn-secondary:hover) {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.15);
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
