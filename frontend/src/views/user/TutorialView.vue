<template>
  <AppLayout>
    <div class="tutorial-page">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <!-- Empty -->
      <div v-else-if="!content" class="flex flex-col items-center justify-center py-20 text-center">
        <svg class="mb-4 h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
        </svg>
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('tutorial.emptyTitle') }}</h3>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('tutorial.emptyDesc') }}</p>
      </div>

      <!-- Content + TOC -->
      <template v-else>
        <div class="tutorial-layout">
          <!-- Left Sticky TOC (desktop) -->
          <aside v-if="tocItems.length" class="tutorial-toc-aside hidden xl:block">
            <nav class="tutorial-toc-sticky">
              <h3 class="toc-title">{{ t('tutorial.toc') }}</h3>
              <ul class="toc-list">
                <li v-for="item in tocItems" :key="item.id">
                  <a
                    :href="'#' + item.id"
                    class="toc-link"
                    :class="[
                      item.level >= 3 ? 'toc-link-sub' : '',
                      activeId === item.id ? 'toc-link-active' : ''
                    ]"
                    @click.prevent="scrollTo(item.id)"
                  >
                    {{ item.text }}
                  </a>
                </li>
              </ul>
            </nav>
          </aside>

          <!-- Main Content -->
          <article
            ref="articleRef"
            class="tutorial-article min-w-0 flex-1 rounded-2xl bg-white p-6 shadow-sm sm:p-8 dark:bg-dark-800"
            v-html="renderedHtml"
            @click="handleArticleClick"
          ></article>
        </div>

        <!-- Mobile Floating TOC Button -->
        <button
          v-if="tocItems.length"
          class="toc-fab xl:hidden"
          @click="mobileTocOpen = !mobileTocOpen"
          :title="t('tutorial.toc')"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12" />
          </svg>
        </button>

        <!-- Mobile TOC Overlay -->
        <transition name="toc-slide">
          <div v-if="mobileTocOpen" class="toc-overlay xl:hidden" @click.self="mobileTocOpen = false">
            <div class="toc-panel">
              <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('tutorial.toc') }}</h3>
                <button @click="mobileTocOpen = false" class="rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                  <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
              <ul class="toc-list p-3">
                <li v-for="item in tocItems" :key="item.id">
                  <a
                    :href="'#' + item.id"
                    class="toc-link"
                    :class="[
                      item.level >= 3 ? 'toc-link-sub' : '',
                      activeId === item.id ? 'toc-link-active' : ''
                    ]"
                    @click.prevent="scrollTo(item.id); mobileTocOpen = false"
                  >
                    {{ item.text }}
                  </a>
                </li>
              </ul>
            </div>
          </div>
        </transition>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked, Renderer } from 'marked'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getUserTutorialContent } from '@/api/tutorialPage'

interface TocItem {
  id: string
  text: string
  level: number
}

const { t } = useI18n()

const content = ref('')
const loading = ref(false)
const activeId = ref('')
const mobileTocOpen = ref(false)
const articleRef = ref<HTMLElement | null>(null)
const tocItems = ref<TocItem[]>([])

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w一-鿿㐀-䶿\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

function stripInlineToc(md: string): string {
  // Remove the "## 目录" or "## Table of Contents" section
  // Match from the TOC heading to the next heading or "---"
  return md.replace(
    /^## 目录\s*\n([\s\S]*?)(?=\n---|\n## [^目])/m,
    ''
  ).replace(
    /^## Table of Contents\s*\n([\s\S]*?)(?=\n---|\n## )/m,
    ''
  ).replace(
    /^- \[.*?\]\(#.*?\)\n/gm,
    ''
  )
}

const renderer = new Renderer()
renderer.heading = function ({ text, depth }: { text: string; depth: number }) {
  const cleanText = text.replace(/<[^>]+>/g, '')
  const id = slugify(cleanText)
  return `<h${depth} id="${id}">${text}</h${depth}>\n`
}

const renderedHtml = computed(() => {
  if (!content.value) return ''
  const cleaned = stripInlineToc(content.value)
  return marked.parse(cleaned, { async: false, renderer }) as string
})

function handleArticleClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  const anchor = target.closest('a')
  if (!anchor) return

  const href = anchor.getAttribute('href')
  if (!href || !href.startsWith('#')) return

  e.preventDefault()
  const id = href.slice(1)
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeId.value = id
  }
}

function buildToc() {
  if (!articleRef.value) return
  const headings = articleRef.value.querySelectorAll('h2, h3')
  const items: TocItem[] = []
  headings.forEach((el) => {
    if (!el.id) return
    items.push({
      id: el.id,
      text: el.textContent || '',
      level: parseInt(el.tagName[1])
    })
  })
  tocItems.value = items
}

function scrollTo(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeId.value = id
  }
}

let observer: IntersectionObserver | null = null

function setupScrollSpy() {
  if (!articleRef.value) return
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          activeId.value = entry.target.id
        }
      }
    },
    { rootMargin: '-80px 0px -70% 0px' }
  )
  const headings = articleRef.value.querySelectorAll('h2, h3')
  headings.forEach((el) => observer!.observe(el))
}

async function load() {
  loading.value = true
  try {
    const data = await getUserTutorialContent()
    content.value = data.content
  } catch (e) {
    console.error('Failed to load tutorial', e)
  } finally {
    loading.value = false
  }
  await nextTick()
  buildToc()
  setupScrollSpy()
}

onMounted(load)
onUnmounted(() => {
  if (observer) observer.disconnect()
})
</script>

<style scoped>
@import '@/assets/tutorial-markdown.css';

/* ===== Layout ===== */
.tutorial-layout {
  display: flex;
  gap: 1.5rem;
  align-items: stretch;
}

/* ===== Left Sticky TOC (desktop) ===== */
.tutorial-toc-aside {
  width: 200px;
  flex-shrink: 0;
  position: relative;
}

.tutorial-toc-sticky {
  position: sticky;
  top: 6rem;
  max-height: calc(100vh - 7rem);
  overflow-y: auto;
  padding-right: 0.25rem;
}

.tutorial-toc-sticky::-webkit-scrollbar {
  width: 3px;
}
.tutorial-toc-sticky::-webkit-scrollbar-thumb {
  background: #d1d5db;
  border-radius: 3px;
}
:root.dark .tutorial-toc-sticky::-webkit-scrollbar-thumb {
  background: #4b5563;
}

.toc-title {
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #9ca3af;
  margin-bottom: 0.5rem;
  padding-left: 0.5rem;
}

.toc-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.toc-link {
  display: block;
  padding: 0.3rem 0.5rem;
  font-size: 0.8125rem;
  line-height: 1.4;
  color: #6b7280;
  border-right: 2px solid transparent;
  border-radius: 0.25rem 0 0 0.25rem;
  transition: all 0.15s;
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toc-link:hover {
  color: #111827;
  background-color: #f3f4f6;
}

:root.dark .toc-link {
  color: #9ca3af;
}
:root.dark .toc-link:hover {
  color: #f9fafb;
  background-color: #1f2937;
}

.toc-link-sub {
  padding-left: 1.25rem;
  font-size: 0.75rem;
}

.toc-link-active {
  color: #4f46e5 !important;
  border-right-color: #4f46e5;
  background-color: #eef2ff;
  font-weight: 500;
}

:root.dark .toc-link-active {
  color: #818cf8 !important;
  border-right-color: #818cf8;
  background-color: rgba(99, 102, 241, 0.1);
}

/* ===== Mobile Floating Button ===== */
.toc-fab {
  position: fixed;
  left: 1.25rem;
  bottom: 1.5rem;
  z-index: 40;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3rem;
  height: 3rem;
  border-radius: 9999px;
  background: #4f46e5;
  color: #fff;
  box-shadow: 0 4px 14px rgba(79, 70, 229, 0.4);
  border: none;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.toc-fab:hover {
  transform: scale(1.05);
  box-shadow: 0 6px 20px rgba(79, 70, 229, 0.5);
}

/* ===== Mobile TOC Overlay ===== */
.toc-overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  justify-content: flex-start;
}

.toc-panel {
  width: 280px;
  max-width: 80vw;
  height: 100%;
  background: #fff;
  overflow-y: auto;
  box-shadow: 4px 0 20px rgba(0, 0, 0, 0.1);
}

:root.dark .toc-panel {
  background: #1f2937;
}

/* Slide transition */
.toc-slide-enter-active,
.toc-slide-leave-active {
  transition: opacity 0.2s ease;
}
.toc-slide-enter-active .toc-panel,
.toc-slide-leave-active .toc-panel {
  transition: transform 0.25s ease;
}
.toc-slide-enter-from {
  opacity: 0;
}
.toc-slide-enter-from .toc-panel {
  transform: translateX(-100%);
}
.toc-slide-leave-to {
  opacity: 0;
}
.toc-slide-leave-to .toc-panel {
  transform: translateX(-100%);
}
</style>
