<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="wide"
    :close-on-escape="false"
    :close-on-click-outside="false"
    @close="emit('cancel')"
  >
    <div class="space-y-5">
      <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-200">
        {{ t(mode === 'login' ? 'legalConsent.loginNotice' : 'legalConsent.registerNotice') }}
      </div>

      <div
        ref="scrollContainer"
        data-testid="legal-consent-scroll"
        class="max-h-[48vh] overflow-y-auto rounded-lg border border-gray-200 bg-white p-4 text-sm leading-7 text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200"
        @scroll="handleScroll"
      >
        <section class="space-y-6">
          <div>
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">
              {{ t('legalConsent.version', { version: resolvedSettings.version }) }}
            </p>
            <h4 class="mt-2 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.contentTitle') }}
            </h4>
            <div class="mt-3 whitespace-pre-wrap">
              {{ resolvedSettings.content }}
            </div>
          </div>
        </section>
      </div>

      <div class="space-y-3">
        <label class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
          <input
            v-model="termsChecked"
            data-testid="legal-consent-terms-check"
            type="checkbox"
            class="mt-1 h-4 w-4"
          />
          <span>{{ t('legalConsent.termsCheck') }}</span>
        </label>

        <label class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
          <input
            v-model="authorizedUseChecked"
            data-testid="legal-consent-authorized-use-check"
            type="checkbox"
            class="mt-1 h-4 w-4"
          />
          <span>{{ t('legalConsent.authorizedUseCheck') }}</span>
        </label>

        <div>
          <label class="input-label">
            {{ t('legalConsent.confirmationLabel') }}
          </label>
          <p class="mb-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('legalConsent.confirmationHint', { phrase: confirmationPhrase }) }}
          </p>
          <textarea
            v-model="typedConfirmation"
            data-testid="legal-consent-confirmation"
            class="input min-h-[86px] resize-none"
            :placeholder="confirmationPhrase"
          ></textarea>
        </div>
      </div>

      <div class="rounded-lg bg-gray-50 px-4 py-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
        <span v-if="remainingSeconds > 0">
          {{ t('legalConsent.countdown', { seconds: remainingSeconds }) }}
        </span>
        <span v-else-if="!scrolledToBottom">
          {{ t('legalConsent.scrollRequired') }}
        </span>
        <span v-else>
          {{ t('legalConsent.ready') }}
        </span>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('cancel')">
        {{ t('common.cancel') }}
      </button>
      <button
        type="button"
        data-testid="legal-consent-confirm"
        class="btn btn-primary"
        :disabled="!canAccept"
        @click="accept"
      >
        {{ t('legalConsent.acceptButton') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  resolveLegalConsentSettings,
  type LegalConsentPayload
} from '@/utils/legalConsent'
import type { LegalConsentSettings } from '@/types'

const props = defineProps<{
  show: boolean
  mode: 'register' | 'login'
  minReadSeconds?: number
  settings?: Partial<LegalConsentSettings> | null
}>()

const emit = defineEmits<{
  (e: 'accept', payload: LegalConsentPayload): void
  (e: 'cancel'): void
}>()

const { t } = useI18n()

const termsChecked = ref(false)
const authorizedUseChecked = ref(false)
const scrolledToBottom = ref(false)
const scrollContainer = ref<HTMLElement | null>(null)
const typedConfirmation = ref('')
const elapsedSeconds = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const resolvedSettings = computed(() => {
  const settings = resolveLegalConsentSettings(props.settings)
  if (props.minReadSeconds !== undefined) {
    return {
      ...settings,
      min_read_seconds: props.minReadSeconds
    }
  }
  return settings
})
const confirmationPhrase = computed(() => resolvedSettings.value.confirmation_phrase)
const dialogTitle = computed(() => (
  props.mode === 'login'
    ? t('legalConsent.loginTitle')
    : t('legalConsent.registerTitle')
))
const remainingSeconds = computed(() => Math.max(0, resolvedSettings.value.min_read_seconds - elapsedSeconds.value))
const canAccept = computed(() => (
  termsChecked.value &&
  authorizedUseChecked.value &&
  scrolledToBottom.value &&
  remainingSeconds.value === 0 &&
  typedConfirmation.value.trim() === confirmationPhrase.value
))

watch(
  () => props.show,
  (show) => {
    stopTimer()
    if (!show) {
      return
    }
    termsChecked.value = false
    authorizedUseChecked.value = false
    scrolledToBottom.value = false
    typedConfirmation.value = ''
    elapsedSeconds.value = 0
    // 内容不足以产生滚动条时不会触发 scroll 事件，需要主动放行，
    // 否则管理员配置较短条款会让接受按钮永远不可用。
    void nextTick().then(syncScrollGate)
    timer = setInterval(() => {
      elapsedSeconds.value += 1
      if (remainingSeconds.value === 0) {
        stopTimer()
      }
    }, 1000)
  },
  { immediate: true }
)

function stopTimer(): void {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function handleScroll(event: Event): void {
  const target = event.target as HTMLElement
  scrolledToBottom.value = target.scrollTop + target.clientHeight >= target.scrollHeight - 4
}

function syncScrollGate(): void {
  const el = scrollContainer.value
  if (!el) {
    return
  }
  if (el.scrollHeight <= el.clientHeight + 4) {
    scrolledToBottom.value = true
  }
}

function accept(): void {
  if (!canAccept.value) {
    return
  }
  emit('accept', {
    typedConfirmation: typedConfirmation.value.trim(),
    dwellSeconds: elapsedSeconds.value,
    scrolledToBottom: scrolledToBottom.value,
    authorizedUseAttestation: authorizedUseChecked.value,
    source: props.mode
  })
}

onBeforeUnmount(stopTimer)
</script>
