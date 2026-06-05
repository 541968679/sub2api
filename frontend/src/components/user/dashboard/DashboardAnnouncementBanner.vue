<template>
  <section
    v-if="announcement"
    class="rounded-lg border border-sky-200 bg-sky-50 px-4 py-3 shadow-sm dark:border-sky-900/50 dark:bg-sky-950/30"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 gap-3">
        <div class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-sky-600 text-white dark:bg-sky-500">
          <Icon name="bell" size="sm" />
        </div>
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-wide text-sky-700 dark:text-sky-300">
            {{ t('dashboard.announcementBannerLabel') }}
          </p>
          <h2 class="mt-0.5 truncate text-sm font-semibold text-gray-950 dark:text-white">
            {{ announcement.title }}
          </h2>
          <p class="mt-1 line-clamp-2 text-sm leading-6 text-gray-700 dark:text-gray-300">
            {{ plainTextContent }}
          </p>
        </div>
      </div>

      <div class="flex flex-shrink-0 items-center gap-2 sm:ml-4">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          @click="showDetail = true"
        >
          <Icon name="eye" size="sm" class="mr-1" />
          {{ t('dashboard.announcementDetails') }}
        </button>
        <button
          type="button"
          class="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-sky-100 hover:text-gray-700 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-sky-900/40 dark:hover:text-gray-200"
          :disabled="dismissing"
          :aria-label="t('dashboard.announcementDismiss')"
          @click="dismiss"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>

    <BaseDialog
      :show="showDetail"
      :title="announcement.title"
      width="wide"
      @close="showDetail = false"
    >
      <div
        class="markdown-body prose prose-sm max-w-none dark:prose-invert"
        v-html="renderedContent"
      ></div>
    </BaseDialog>
  </section>
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

const { t } = useI18n()

const announcement = ref<UserAnnouncement | null>(null)
const showDetail = ref(false)
const dismissing = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true
})

const renderedContent = computed(() => {
  if (!announcement.value?.content) return ''
  return DOMPurify.sanitize(marked.parse(announcement.value.content) as string)
})

const plainTextContent = computed(() => {
  if (!announcement.value?.content) return ''
  return announcement.value.content
    .replace(/[#>*_`[\]()!-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
})

async function loadAnnouncement() {
  const items = await announcementsAPI.list({ surface: 'dashboard_banner' })
  announcement.value = items.find((item) => !item.banner_dismissed_at) || null
}

async function dismiss() {
  if (!announcement.value) return
  dismissing.value = true
  try {
    await announcementsAPI.dismissBanner(announcement.value.id)
    announcement.value = null
    showDetail.value = false
  } catch (error) {
    console.error('Failed to dismiss dashboard announcement banner:', error)
  } finally {
    dismissing.value = false
  }
}

onMounted(() => {
  loadAnnouncement()
})
</script>
