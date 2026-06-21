<template>
  <div class="rounded-xl border border-primary-200 bg-white/90 px-4 py-3 shadow-sm dark:border-primary-800/40 dark:bg-dark-800/80">
    <!-- Header row: title + rules + dismiss (kept on one line to minimize height) -->
    <div class="mb-2.5 flex items-center gap-3">
      <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/40">
        <Icon name="book" size="md" class="text-primary-600 dark:text-primary-400" />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.title') }}</h3>
        <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ t('keys.guide.subtitle') }}</p>
      </div>

      <a
        v-if="tutorialUrl"
        :href="tutorialUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="guide-tutorial"
      >
        <Icon name="externalLink" size="sm" />
        <span class="hidden sm:inline">{{ t('keys.guide.detailedTutorial') }}</span>
      </a>

      <button
        v-if="rulesAnnouncement"
        type="button"
        class="guide-rule"
        @click="showRules = true"
      >
        <Icon name="shield" size="sm" />
        <span class="hidden sm:inline">{{ t('keys.guide.rulesTitle') }}</span>
        <span class="guide-rule-action">{{ t('keys.guide.rulesAction') }}</span>
      </button>

      <button
        @click="emit('dismiss')"
        class="shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
        :title="t('keys.guide.dismiss')"
      >
        <Icon name="x" size="sm" />
      </button>
    </div>

    <!-- Step cards -->
    <div
      class="grid grid-cols-1 gap-3"
      :class="hideCcsImport ? 'sm:grid-cols-2' : 'sm:grid-cols-3'"
    >
      <!-- Step 1: Create API Key -->
      <div class="guide-card">
        <span class="guide-card-num">1</span>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5">
            <Icon name="key" size="sm" class="shrink-0 text-primary-500 dark:text-primary-400" />
            <h4 class="guide-card-title">{{ t('keys.guide.step1Title') }}</h4>
          </div>
          <p class="guide-card-desc">{{ t('keys.guide.step1Desc') }}</p>
          <div class="guide-card-action">
            <button
              v-if="!hasKeys"
              @click="emit('createKey')"
              class="btn btn-primary h-8 px-2.5 text-xs"
            >
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('keys.guide.step1Action') }}
            </button>
            <span
              v-else
              class="inline-flex items-center gap-1.5 text-xs font-medium text-green-600 dark:text-green-400"
            >
              <Icon name="checkCircle" size="sm" />
              {{ t('common.done') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Step 2: Install CC Switch (hidden if admin disabled CCS) -->
      <div v-if="!hideCcsImport" class="guide-card">
        <span class="guide-card-num">2</span>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5">
            <Icon name="download" size="sm" class="shrink-0 text-primary-500 dark:text-primary-400" />
            <h4 class="guide-card-title">{{ t('keys.guide.step2Title') }}</h4>
          </div>
          <p class="guide-card-desc">{{ t('keys.guide.step2Desc') }}</p>
          <div class="guide-card-action flex-wrap">
            <a
              :href="ccsWindowsUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary inline-flex h-8 items-center gap-1.5 px-2.5 text-xs"
            >
              <Icon name="download" size="sm" />
              Windows
            </a>
            <a
              :href="ccsMacUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary inline-flex h-8 items-center gap-1.5 px-2.5 text-xs"
            >
              <Icon name="download" size="sm" />
              macOS
            </a>
          </div>
        </div>
      </div>

      <!-- Step 3: Use Your Key -->
      <div class="guide-card">
        <span class="guide-card-num">{{ hideCcsImport ? 2 : 3 }}</span>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5">
            <Icon name="terminal" size="sm" class="shrink-0 text-primary-500 dark:text-primary-400" />
            <h4 class="guide-card-title">{{ t('keys.guide.step3Title') }}</h4>
          </div>
          <p class="guide-card-desc">
            {{ hideCcsImport ? t('keys.guide.step3DescNoCcs') : t('keys.guide.step3Desc') }}
          </p>
          <div class="guide-card-action text-xs text-gray-500 dark:text-gray-400">
            <span class="inline-flex items-center gap-1">
              <Icon name="terminal" size="sm" />Claude Code
            </span>
            <span class="inline-flex items-center gap-1">
              <Icon name="sparkles" size="sm" />Gemini CLI
            </span>
          </div>
        </div>
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
  /** External (Feishu) tutorial link; when set, a "详细教程" button is shown. */
  tutorialUrl?: string
}

interface Emits {
  (e: 'createKey'): void
  (e: 'dismiss'): void
}

const props = defineProps<Props>()
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

// ===== CC Switch download links =====
// File names embed the version (e.g. CC-Switch-v3.16.3-Windows.msi), so there is
// no stable "latest" asset URL. Resolve the newest release from the GitHub API at
// runtime (cached 24h), and fall back to the releases page on any failure.
const CCS_RELEASES_PAGE = 'https://github.com/farion1231/cc-switch/releases'
const CCS_LATEST_API = 'https://api.github.com/repos/farion1231/cc-switch/releases/latest'
const CCS_CACHE_KEY = 'sub2api_ccs_release'
const CCS_CACHE_TTL = 24 * 60 * 60 * 1000 // 24h

const ccsWindowsUrl = ref(CCS_RELEASES_PAGE)
const ccsMacUrl = ref(CCS_RELEASES_PAGE)

interface CcsReleaseCache {
  ts: number
  win: string
  mac: string
}

interface GithubAsset {
  name: string
  browser_download_url: string
}

function applyAssets(assets: GithubAsset[]) {
  const win = assets.find((a) => /windows/i.test(a.name) && /\.(msi|exe)$/i.test(a.name))
  const mac = assets.find((a) => /macos/i.test(a.name) && /\.dmg$/i.test(a.name))
  if (win) ccsWindowsUrl.value = win.browser_download_url
  if (mac) ccsMacUrl.value = mac.browser_download_url
}

async function loadCcsRelease() {
  // Serve from cache when fresh to avoid GitHub's 60 req/h/IP unauthenticated limit.
  try {
    const raw = localStorage.getItem(CCS_CACHE_KEY)
    if (raw) {
      const cached = JSON.parse(raw) as CcsReleaseCache
      if (cached?.win && cached?.mac && Date.now() - cached.ts < CCS_CACHE_TTL) {
        ccsWindowsUrl.value = cached.win
        ccsMacUrl.value = cached.mac
        return
      }
    }
  } catch {
    // ignore malformed cache
  }

  try {
    const resp = await fetch(CCS_LATEST_API, {
      headers: { Accept: 'application/vnd.github+json' }
    })
    if (!resp.ok) throw new Error(`GitHub API responded ${resp.status}`)
    const data = (await resp.json()) as { assets?: GithubAsset[] }
    applyAssets(data.assets || [])
    localStorage.setItem(
      CCS_CACHE_KEY,
      JSON.stringify({ ts: Date.now(), win: ccsWindowsUrl.value, mac: ccsMacUrl.value })
    )
  } catch (error) {
    // Keep the releases-page fallback so the buttons are never dead links.
    console.warn('Failed to resolve CC Switch latest release; using releases page', error)
  }
}

onMounted(() => {
  loadRules()
  if (!props.hideCcsImport) {
    loadCcsRelease()
  }
})
</script>

<style scoped>
.guide-card {
  @apply flex gap-2.5 rounded-lg border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-900/40;
}

.guide-card-num {
  @apply mt-px flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary-500 text-[11px] font-bold text-white;
}

.guide-card-title {
  @apply truncate text-sm font-semibold text-gray-900 dark:text-white;
}

.guide-card-desc {
  @apply mt-1 line-clamp-2 text-xs leading-snug text-gray-500 dark:text-gray-400;
}

.guide-card-action {
  @apply mt-2 flex items-center gap-2;
}

.guide-tutorial {
  @apply inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-primary-200 bg-primary-50/80 px-2.5 py-1.5 text-xs font-medium text-primary-700 transition-colors hover:bg-primary-100 dark:border-primary-800/50 dark:bg-primary-950/30 dark:text-primary-300 dark:hover:bg-primary-900/40;
}

.guide-rule {
  @apply inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50/80 px-2.5 py-1.5 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300 dark:hover:bg-amber-900/40;
}

.guide-rule-action {
  @apply rounded bg-amber-200/60 px-1.5 py-0.5 text-[11px] dark:bg-amber-900/40;
}
</style>
