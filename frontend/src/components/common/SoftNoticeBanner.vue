<template>
  <div
    v-if="text"
    class="flex items-start gap-3 rounded-xl border-2 border-emerald-300 bg-emerald-50 px-4 py-4 text-emerald-900 shadow-sm dark:border-emerald-600/50 dark:bg-emerald-950/35 dark:text-emerald-100 sm:px-5 sm:py-5"
    role="status"
  >
    <span
      class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300"
    >
      <Icon name="infoCircle" size="md" />
    </span>
    <div class="min-w-0 flex-1">
      <p
        v-if="title"
        class="text-sm font-semibold tracking-wide text-emerald-800 dark:text-emerald-200 sm:text-base"
      >
        {{ title }}
      </p>
      <p
        class="whitespace-pre-line font-semibold leading-8 text-lg sm:text-xl"
        :class="title ? 'mt-1.5' : ''"
      >
        <template v-if="linkify">
          <template v-for="(part, index) in linkedParts" :key="index">
            <a
              v-if="part.href"
              :href="part.href"
              target="_blank"
              rel="noopener noreferrer"
              class="break-all underline decoration-emerald-400/80 underline-offset-4 hover:decoration-emerald-700"
            >{{ part.text }}</a>
            <template v-else>{{ part.text }}</template>
          </template>
        </template>
        <template v-else>{{ text }}</template>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { linkifyHttpText } from '@/utils/httpTextLinks'

const props = withDefaults(defineProps<{
  text: string
  title?: string
  /** Turn the first safe http(s) URLs into links. Plain text stays escaped. */
  linkify?: boolean
}>(), {
  linkify: false,
})

const linkedParts = computed(() => linkifyHttpText(props.text))
</script>
