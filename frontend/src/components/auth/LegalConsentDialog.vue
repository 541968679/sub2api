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
        data-testid="legal-consent-scroll"
        class="max-h-[48vh] overflow-y-auto rounded-lg border border-gray-200 bg-white p-4 text-sm leading-7 text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200"
        @scroll="handleScroll"
      >
        <section class="space-y-6">
          <div>
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">
              {{ t('legalConsent.version', { version: CURRENT_LEGAL_CONSENT_VERSION }) }}
            </p>
            <h4 class="mt-2 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.disclaimerTitle') }}
            </h4>
            <p class="mt-2">
              {{ t('legalConsent.disclaimerBody') }}
            </p>
          </div>

          <div>
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.regionTitle') }}
            </h4>
            <p class="mt-2">
              {{ t('legalConsent.regionBody') }}
            </p>
          </div>

          <div>
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.prohibitedTitle') }}
            </h4>
            <ul class="mt-2 list-disc space-y-2 pl-5">
              <li v-for="item in prohibitedItems" :key="item">
                {{ item }}
              </li>
            </ul>
          </div>

          <div>
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.enforcementTitle') }}
            </h4>
            <p class="mt-2">
              {{ t('legalConsent.enforcementBody') }}
            </p>
          </div>

          <div>
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.accountTitle') }}
            </h4>
            <p class="mt-2">
              {{ t('legalConsent.accountBody') }}
            </p>
          </div>

          <div>
            <h4 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('legalConsent.availabilityTitle') }}
            </h4>
            <p class="mt-2">
              {{ t('legalConsent.availabilityBody') }}
            </p>
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
            v-model="regionChecked"
            data-testid="legal-consent-region-check"
            type="checkbox"
            class="mt-1 h-4 w-4"
          />
          <span>{{ t('legalConsent.regionCheck') }}</span>
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
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  CURRENT_LEGAL_CONSENT_VERSION,
  type LegalConsentPayload
} from '@/utils/legalConsent'

const props = withDefaults(defineProps<{
  show: boolean
  mode: 'register' | 'login'
  minReadSeconds?: number
}>(), {
  minReadSeconds: 20
})

const emit = defineEmits<{
  (e: 'accept', payload: LegalConsentPayload): void
  (e: 'cancel'): void
}>()

const { t } = useI18n()

const termsChecked = ref(false)
const regionChecked = ref(false)
const scrolledToBottom = ref(false)
const typedConfirmation = ref('')
const elapsedSeconds = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const confirmationPhrase = computed(() => t('legalConsent.confirmationPhrase'))
const dialogTitle = computed(() => (
  props.mode === 'login'
    ? t('legalConsent.loginTitle')
    : t('legalConsent.registerTitle')
))
const prohibitedItems = computed(() => [
  t('legalConsent.prohibited.nsfw'),
  t('legalConsent.prohibited.violence'),
  t('legalConsent.prohibited.minors'),
  t('legalConsent.prohibited.sillyTavern'),
  t('legalConsent.prohibited.privacy'),
  t('legalConsent.prohibited.fraud'),
  t('legalConsent.prohibited.security'),
  t('legalConsent.prohibited.illegal')
])
const remainingSeconds = computed(() => Math.max(0, props.minReadSeconds - elapsedSeconds.value))
const canAccept = computed(() => (
  termsChecked.value &&
  regionChecked.value &&
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
    regionChecked.value = false
    scrolledToBottom.value = false
    typedConfirmation.value = ''
    elapsedSeconds.value = 0
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

function accept(): void {
  if (!canAccept.value) {
    return
  }
  emit('accept', {
    typedConfirmation: typedConfirmation.value.trim(),
    dwellSeconds: elapsedSeconds.value,
    scrolledToBottom: scrolledToBottom.value,
    regionAttestation: regionChecked.value,
    source: props.mode
  })
}

onBeforeUnmount(stopTimer)
</script>
