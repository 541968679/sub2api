<template>
  <div class="rounded-xl border border-primary-200 bg-white/90 px-4 py-3 shadow-sm dark:border-primary-800/40 dark:bg-dark-800/80">
    <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
      <div class="flex min-w-0 items-center gap-3">
        <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/40">
          <Icon name="book" size="md" class="text-primary-600 dark:text-primary-400" />
        </div>
        <div class="min-w-0">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.title') }}</h3>
          <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ t('keys.guide.subtitle') }}</p>
        </div>
      </div>

      <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center xl:justify-end">
        <button
          v-if="rulesAnnouncement"
          type="button"
          class="guide-rule"
          @click="showRules = true"
        >
          <span class="guide-rule-icon">
            <Icon name="shield" size="sm" />
          </span>
          <span class="guide-step-title">{{ t('keys.guide.rulesTitle') }}</span>
          <span class="guide-rule-action">{{ t('keys.guide.rulesAction') }}</span>
        </button>

        <!-- Step 1: Create API Key -->
        <div class="guide-step">
          <span class="guide-step-number">1</span>
          <span class="guide-step-title">{{ t('keys.guide.step1Title') }}</span>
          <button
            v-if="!hasKeys"
            @click="emit('createKey')"
            class="btn btn-primary h-8 px-2.5 text-xs"
          >
            <Icon name="plus" size="sm" class="mr-1" />
            {{ t('keys.guide.step1Action') }}
          </button>
          <div v-else class="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-400">
            <Icon name="checkCircle" size="sm" />
            <span>{{ t('common.done') }}</span>
          </div>
        </div>

        <!-- Step 2: Install CC Switch (hidden if admin disabled CCS) -->
        <div v-if="!hideCcsImport" class="guide-step">
          <span class="guide-step-number">2</span>
          <span class="guide-step-title">{{ t('keys.guide.step2Title') }}</span>
          <a
            href="https://github.com/farion1231/cc-switch/releases"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary inline-flex h-8 items-center gap-1.5 px-2.5 text-xs"
          >
            <Icon name="externalLink" size="sm" />
            {{ t('keys.guide.step2Download') }}
          </a>
        </div>

        <!-- Step 3: Use Your Key -->
        <div class="guide-step">
          <span class="guide-step-number">{{ hideCcsImport ? 2 : 3 }}</span>
          <span class="guide-step-title">{{ t('keys.guide.step3Title') }}</span>
          <div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
            <div class="flex items-center gap-1">
              <Icon name="terminal" size="sm" />
              <span>Claude Code</span>
            </div>
            <div class="flex items-center gap-1">
              <Icon name="sparkles" size="sm" />
              <span>Gemini CLI</span>
            </div>
          </div>
        </div>

        <button
          @click="emit('dismiss')"
          class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
          :title="t('keys.guide.dismiss')"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>

    <BaseDialog
      :show="showRules"
      :title="rulesAnnouncement?.title || t('keys.guide.rulesModalTitle')"
      width="wide"
      @close="showRules = false"
    >
      <div
        class="markdown-body prose prose-sm max-w-none dark:prose-invert"
        v-html="renderedRules"
      ></div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { announcementsAPI } from '@/api'
import type { UserAnnouncement } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  hasKeys: boolean
  hideCcsImport?: boolean
}

interface Emits {
  (e: 'createKey'): void
  (e: 'dismiss'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()

const rulesAnnouncement = ref<UserAnnouncement | null>(null)
const showRules = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true
})

const renderedRules = computed(() => {
  if (!rulesAnnouncement.value?.content) return ''
  return DOMPurify.sanitize(marked.parse(rulesAnnouncement.value.content) as string)
})

async function loadRules() {
  try {
    const items = await announcementsAPI.list({ surface: 'api_key_rules' })
    rulesAnnouncement.value = items[0] || null
  } catch (error) {
    console.error('Failed to load API key usage rules:', error)
  }
}

onMounted(() => {
  loadRules()
})
</script>

<style scoped>
.guide-step {
  @apply flex min-h-10 min-w-0 items-center gap-2 rounded-lg border border-gray-200 bg-gray-50/80 px-3 py-1.5 dark:border-dark-700 dark:bg-dark-900/40;
}

.guide-step-number {
  @apply flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary-500 text-[11px] font-bold text-white;
}

.guide-step-title {
  @apply shrink-0 text-sm font-medium text-gray-900 dark:text-white;
}

.guide-rule {
  @apply flex min-h-10 min-w-0 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-1.5 text-left transition-colors hover:bg-amber-100 dark:border-amber-900/50 dark:bg-amber-950/30 dark:hover:bg-amber-900/40;
}

.guide-rule-icon {
  @apply flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-500 text-white;
}

.guide-rule-action {
  @apply shrink-0 text-xs font-medium text-amber-700 dark:text-amber-300;
}
</style>
