<template>
  <div class="rounded-2xl border border-primary-200 bg-gradient-to-br from-primary-50 via-white to-blue-50 dark:border-primary-800/40 dark:from-primary-950/40 dark:via-dark-800 dark:to-blue-950/30">
    <!-- Header -->
    <div class="flex items-center justify-between px-5 py-4 border-b border-primary-100 dark:border-primary-800/30">
      <div class="flex items-center gap-3">
        <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/40">
          <Icon name="book" size="md" class="text-primary-600 dark:text-primary-400" />
        </div>
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.title') }}</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.guide.subtitle') }}</p>
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

    <!-- Steps -->
    <div class="p-5">
      <div :class="[
        'grid gap-4',
        hideCcsImport ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1 md:grid-cols-3'
      ]">
        <!-- Step 1: Create API Key -->
        <div class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800/60">
          <div class="flex items-center gap-2">
            <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-500 text-xs font-bold text-white">1</span>
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.step1Title') }}</span>
          </div>
          <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">
            {{ t('keys.guide.step1Desc') }}
          </p>
          <button
            v-if="!hasKeys"
            @click="emit('createKey')"
            class="btn btn-primary mt-auto self-start text-xs px-3 py-1.5"
          >
            <Icon name="plus" size="sm" class="mr-1" />
            {{ t('keys.guide.step1Action') }}
          </button>
          <div v-else class="mt-auto flex items-center gap-1.5 text-xs text-green-600 dark:text-green-400">
            <Icon name="checkCircle" size="sm" />
            <span>{{ t('common.done') }}</span>
          </div>
        </div>

        <!-- Step 2: Install CC Switch (hidden if admin disabled CCS) -->
        <div v-if="!hideCcsImport" class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800/60">
          <div class="flex items-center gap-2">
            <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-500 text-xs font-bold text-white">2</span>
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.step2Title') }}</span>
          </div>
          <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">
            {{ t('keys.guide.step2Desc') }}
          </p>
          <a
            href="https://github.com/farion1231/cc-switch/releases"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary mt-auto self-start text-xs px-3 py-1.5 inline-flex items-center gap-1.5"
          >
            <Icon name="externalLink" size="sm" />
            {{ t('keys.guide.step2Download') }}
          </a>
        </div>

        <!-- Step 3: Use Your Key -->
        <div class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800/60">
          <div class="flex items-center gap-2">
            <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-500 text-xs font-bold text-white">{{ hideCcsImport ? 2 : 3 }}</span>
            <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('keys.guide.step3Title') }}</span>
          </div>
          <p class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">
            {{ hideCcsImport ? t('keys.guide.step3DescNoCcs') : t('keys.guide.step3Desc') }}
          </p>
          <div class="mt-auto flex items-center gap-3 text-xs text-gray-400 dark:text-gray-500">
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
      </div>

      <!-- Dismiss link -->
      <div class="mt-3 text-center">
        <button
          @click="emit('dismiss')"
          class="text-xs text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-300"
        >
          {{ t('keys.guide.dismiss') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
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
</script>
