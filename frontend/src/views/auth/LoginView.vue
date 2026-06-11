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
            <router-link
              to="/key-usage"
              class="rounded-[17px] bg-white/[0.06] px-5 py-2 text-[13px] font-semibold text-[#DCE7F2] transition-colors hover:bg-white/[0.10]"
            >
              {{ t('auth.login.navKeyUsage') }}
            </router-link>
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
          <div class="hidden flex-1 flex-col gap-8 rounded-l-[32px] bg-gradient-to-br from-[#18D8AA]/[0.16] to-[#4BA8FF]/[0.04] border border-white/[0.08] p-8 lg:flex xl:gap-10 xl:p-10">
            <!-- Badge + Heading -->
            <div>
              <span class="inline-block rounded-[20px] bg-[#ECFFF9] px-4 py-2 text-[13px] font-extrabold tracking-wide text-[#0D2A3C]">
                {{ loginBadge }}
              </span>

              <h1 class="mt-6 text-[40px] font-extrabold leading-[1.15] text-white xl:text-[48px]">
                {{ loginHeading1 }}
              </h1>
              <h1 class="text-[40px] font-extrabold leading-[1.15] text-[#9BFFEA] xl:text-[48px]">
                {{ loginHeading2 }}
              </h1>
            </div>

            <!-- Bottom section: 4 equal feature cards in a 2×2 grid -->
            <div class="grid auto-rows-fr gap-5 sm:grid-cols-2 xl:gap-6">
              <div
                v-for="card in featureCards"
                :key="card.key"
                class="group relative flex min-h-[188px] flex-col overflow-hidden rounded-[22px] border border-[#2F5672] bg-gradient-to-br from-[#102A40] via-[#0D2031] to-[#081827] p-7 shadow-[0_12px_34px_rgba(0,0,0,0.26)] transition-colors hover:border-[#60A5C9]"
              >
                <!-- 顶部光带：每张卡的主题色从左渐变消失，视觉上能一眼识别各自代表什么 -->
                <div class="absolute inset-x-0 top-0 h-[2px]" :class="card.topStripe"></div>

                <!-- 标题行：较大图标（48×48）+ 19px 粗标题 -->
                <div class="flex items-center gap-3">
                  <span
                    class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl"
                    :class="[card.iconBg, card.iconColor]"
                  >
                    <svg class="h-6 w-6" fill="none" stroke="currentColor" stroke-width="1.8" viewBox="0 0 24 24" aria-hidden="true">
                      <path stroke-linecap="round" stroke-linejoin="round" :d="card.iconPath" />
                    </svg>
                  </span>
                  <h3 class="text-[19px] font-extrabold leading-snug text-white">{{ card.title }}</h3>
                </div>

                <!-- 描述：15px，关键词用主题色 + 加粗 突出展示 -->
                <p class="mt-5 text-[15px] leading-[1.75] text-[#C8D7E4]">
                  <template v-for="(seg, i) in card.segments" :key="i">
                    <span
                      v-if="seg.highlight"
                      class="font-extrabold"
                      :class="card.highlightColor"
                    >{{ seg.text }}</span>
                    <template v-else>{{ seg.text }}</template>
                  </template>
                </p>
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
              <h2 class="text-[28px] font-extrabold text-[#F6FBFF]">{{ loginFormTitle }}</h2>
              <p class="mt-2 text-[14px] text-[#8EA6BD]">{{ loginFormSubtitle }}</p>

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
                <router-link
                  to="/key-usage"
                  class="mt-3 inline-flex text-sm font-bold text-[#7FE9D4] transition-colors hover:text-[#9BFFEA]"
                >
                  {{ t('auth.login.keyUsageLink') }}
                </router-link>
              </div>

              <!-- OAuth Section -->
              <div v-if="!backendModeEnabled && (linuxdoOAuthEnabled || wechatOAuthEnabled || oidcOAuthEnabled)" class="mt-6">
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
import type { LegalConsentSettings, TotpLoginResponse, User } from '@/types'
import { clearAllAffiliateReferralCodes } from '@/utils/oauthAffiliate'
import {
  hasAcceptedCurrentLegalConsent,
  markLegalConsentAccepted,
  resolveLegalConsentSettings,
  type LegalConsentPayload
} from '@/utils/legalConsent'

const { t, locale } = useI18n()

// ==================== Router & Stores ====================

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

// ==================== Site Info (replaces AuthLayout) ====================

const siteName = computed(() => appStore.siteName || 'ZeroCode')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const docUrl = computed(() => appStore.docUrl)

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
const loginFormTitle = computed(() => pickLoginText(loginPageOverrides.value?.form_title, t('auth.login.title')))
const loginFormSubtitle = computed(() => pickLoginText(loginPageOverrides.value?.form_subtitle, t('auth.login.subtitle')))

// ==================== Feature Cards (2×2 grid) ====================
// 卡片文字来自 i18n（`auth.login.features.*.title` / `.desc`）。
// 这里声明的都是「视觉规则」：
//   - 每张卡的主题色（图标、顶部光带、描述里高亮词的颜色）
//   - 图标 SVG path（heroicons outline）
//   - 要在描述里突出展示的关键词（不同语言各配一套；匹配不到就原样显示）
// 加强版样式可以让价格、Opus/GPT/Gemini 型号名、gpt-image-2 等一眼能看到。

type FeatureKey = 'metered' | 'models' | 'image' | 'tutorial'

const escapeRegExp = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

function splitWithTerms(text: string, terms: readonly string[]): Array<{ text: string; highlight: boolean }> {
  const sorted = terms.filter(Boolean).slice().sort((a, b) => b.length - a.length)
  if (sorted.length === 0) return [{ text, highlight: false }]
  const pattern = new RegExp(`(${sorted.map(escapeRegExp).join('|')})`, 'g')
  return text
    .split(pattern)
    .filter((part) => part !== '')
    .map((part) => ({ text: part, highlight: sorted.includes(part) }))
}

// 每张卡在不同语言下需要高亮的子串。更动 i18n 文案后若找不到子串，则不加高亮，
// 不会崩溃——高亮是锦上添花，不影响可读性。
const featureHighlightTermsZh: Record<FeatureKey, readonly string[]> = {
  metered: ['0.7 元', '1/10', '超高性价比'],
  models: ['Opus 4.7', 'GPT-5.4', 'Gemini 3.1 Pro'],
  image: ['gpt-image-2', '生图', '高质量图片'],
  tutorial: ['完整', '高可读性', '快速上手']
}
const featureHighlightTermsEn: Record<FeatureKey, readonly string[]> = {
  metered: ['0.7 CNY', '1/10', 'excellent value'],
  models: ['Opus 4.7', 'GPT-5.4', 'Gemini 3.1 Pro'],
  image: ['gpt-image-2', 'image generation', 'high-quality images'],
  tutorial: ['Complete', 'readable', 'productive fast']
}

interface FeatureCardDef {
  key: FeatureKey
  /** Tailwind class: 图标圆底色 */
  iconBg: string
  /** Tailwind class: 图标描边色 */
  iconColor: string
  /** Tailwind class: 描述里高亮词的颜色 */
  highlightColor: string
  /** Tailwind class: 卡片顶部渐变光带 */
  topStripe: string
  /** heroicon outline path data */
  iconPath: string
  title: string
  segments: Array<{ text: string; highlight: boolean }>
}

const featureCards = computed<FeatureCardDef[]>(() => {
  const terms = locale.value.startsWith('en') ? featureHighlightTermsEn : featureHighlightTermsZh
  const defs: Array<Omit<FeatureCardDef, 'title' | 'segments'> & { key: FeatureKey }> = [
    {
      key: 'metered',
      iconBg: 'bg-[#18D8AA]/15',
      iconColor: 'text-[#7CF5CC]',
      highlightColor: 'text-[#9BFFEA]',
      topStripe: 'bg-gradient-to-r from-[#18D8AA]/70 via-[#18D8AA]/20 to-transparent',
      // currency (dollar) with circle
      iconPath:
        'M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-2.21 0-4-1.79-4-4s1.79-4 4-4 4 1.79 4 4'
    },
    {
      key: 'models',
      iconBg: 'bg-[#A78BFA]/15',
      iconColor: 'text-[#C4B5FD]',
      highlightColor: 'text-[#DDD0FF]',
      topStripe: 'bg-gradient-to-r from-[#A78BFA]/70 via-[#A78BFA]/20 to-transparent',
      // sparkles
      iconPath:
        'M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z'
    },
    {
      key: 'image',
      iconBg: 'bg-[#4BA8FF]/15',
      iconColor: 'text-[#91D5FF]',
      highlightColor: 'text-[#B7E7FF]',
      topStripe: 'bg-gradient-to-r from-[#4BA8FF]/70 via-[#22D3EE]/25 to-transparent',
      // photo
      iconPath:
        'M2.25 15.75l5.159-5.159a2.25 2.25 0 013.182 0l5.159 5.159m-1.5-1.5l1.409-1.409a2.25 2.25 0 013.182 0l2.909 2.909M3.75 19.5h16.5A1.5 1.5 0 0021.75 18V6a1.5 1.5 0 00-1.5-1.5H3.75A1.5 1.5 0 002.25 6v12a1.5 1.5 0 001.5 1.5zm10.5-11.25h.008v.008h-.008V8.25z'
    },
    {
      key: 'tutorial',
      iconBg: 'bg-[#22D3EE]/15',
      iconColor: 'text-[#7DE5F5]',
      highlightColor: 'text-[#A9F0F9]',
      topStripe: 'bg-gradient-to-r from-[#22D3EE]/70 via-[#22D3EE]/20 to-transparent',
      // book-open
      iconPath:
        'M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25'
    }
  ]
  return defs.map<FeatureCardDef>((d) => ({
    ...d,
    title: t(`auth.login.features.${d.key}.title`),
    segments: splitWithTerms(t(`auth.login.features.${d.key}.desc`), terms[d.key])
  }))
})

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
