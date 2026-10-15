<template>
  <span
    class="inline-flex shrink-0 items-center justify-center rounded-md"
    :class="[boxClass, toneClass]"
    :title="title"
    aria-hidden="true"
  >
    <ModelIcon v-if="modelName" :model="modelName" :size="iconSize" />
    <svg v-else :class="iconClass" viewBox="0 0 24 24" fill="currentColor">
      <path d="M4 4h6.5v6.5H4V4zm9.5 0H20v6.5h-6.5V4zM4 13.5h6.5V20H4v-6.5zm9.5 0H20V20h-6.5v-6.5z" />
    </svg>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { isDomesticGroupCategory, type DomesticGroupCategory } from '@/utils/domesticGroupCategory'

const props = withDefaults(defineProps<{
  category?: string | null
  size?: 'sm' | 'md'
}>(), {
  size: 'sm'
})

const category = computed<DomesticGroupCategory>(() =>
  isDomesticGroupCategory(props.category) ? props.category : 'mixed'
)

const title = computed(() => category.value)

const modelName = computed(() => {
  switch (category.value) {
    case 'kimi':
      return 'kimi'
    case 'glm':
      return 'glm'
    case 'qwen':
      return 'qwen'
    case 'deepseek':
      return 'deepseek'
    default:
      return ''
  }
})

const boxClass = computed(() => (props.size === 'md' ? 'h-6 w-6' : 'h-5 w-5'))
const iconClass = computed(() => (props.size === 'md' ? 'h-3.5 w-3.5' : 'h-3 w-3'))
const iconSize = computed(() => (props.size === 'md' ? '16px' : '14px'))

const toneClass = computed(() => {
  if (category.value === 'mixed') {
    return 'bg-violet-100 text-violet-700 dark:bg-violet-900/50 dark:text-violet-200'
  }
  return 'bg-white ring-1 ring-black/10 dark:bg-white'
})
</script>
