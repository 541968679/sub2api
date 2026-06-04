<template>
  <div
    v-if="contactInfo"
    class="flex min-h-11 items-center justify-between gap-3 rounded-lg border border-sky-100 bg-sky-50/70 px-3 py-2 text-sky-900 dark:border-sky-900/40 dark:bg-sky-950/30 dark:text-sky-100 sm:px-4"
    role="note"
  >
    <div class="flex min-w-0 items-center gap-2">
      <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-white text-sky-600 shadow-sm dark:bg-sky-900/50 dark:text-sky-300">
        <Icon name="chat" size="sm" />
      </span>
      <div class="min-w-0 text-sm leading-5 sm:flex sm:items-center sm:gap-1">
        <span class="shrink-0 font-medium">{{ label }}</span>
        <span class="hidden text-sky-400 dark:text-sky-500 sm:inline">:</span>
        <span class="block min-w-0 truncate font-semibold text-sky-950 dark:text-sky-50" :title="contactInfo">
          {{ contactInfo }}
        </span>
      </div>
    </div>

    <button
      type="button"
      class="inline-flex h-8 shrink-0 items-center justify-center gap-1.5 rounded-md border border-sky-200 bg-white px-2.5 text-xs font-medium text-sky-700 transition-colors hover:border-sky-300 hover:bg-sky-100 dark:border-sky-800 dark:bg-sky-900/50 dark:text-sky-200 dark:hover:bg-sky-900"
      :title="t('supportContact.copy')"
      :aria-label="t('supportContact.copy')"
      @click="copyContact"
    >
      <Icon :name="copied ? 'check' : 'copy'" size="xs" />
      <span class="hidden sm:inline">{{ copied ? t('supportContact.copied') : t('common.copy') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  context?: 'dashboard' | 'payment'
}>(), {
  context: 'dashboard'
})

const { t } = useI18n()
const appStore = useAppStore()
const { copied, copyToClipboard } = useClipboard()

const contactInfo = computed(() => appStore.contactInfo.trim())
const label = computed(() => (
  props.context === 'payment'
    ? t('supportContact.paymentLabel')
    : t('supportContact.dashboardLabel')
))

function copyContact() {
  void copyToClipboard(contactInfo.value, t('supportContact.copySuccess'))
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    void appStore.fetchPublicSettings()
  }
})
</script>
