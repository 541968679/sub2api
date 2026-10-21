<template>
  <div
    :class="
      sticky
        ? 'sticky top-16 z-20 mb-3 bg-gray-50 pb-1 dark:bg-dark-950 lg:hidden'
        : 'mb-3'
    "
  >
    <div
      class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-xl border border-sky-200 bg-sky-50 px-4 py-2.5 shadow-sm dark:border-sky-800/50 dark:bg-sky-950/30"
      :data-testid="testId"
    >
      <span class="text-sm font-semibold text-sky-950 dark:text-sky-100">
        {{ label }}
      </span>
      <span
        class="font-mono text-sm font-semibold tabular-nums text-sky-800 dark:text-sky-200"
        :data-testid="rateTestId"
        :data-rate="amount"
      >
        {{ display }}
      </span>
      <span class="text-xs text-sky-800/80 dark:text-sky-200/80">
        {{ countLabel }}
      </span>
      <HelpTooltip :content="hint" width-class="w-80" />
    </div>
  </div>
</template>

<script setup lang="ts">
import HelpTooltip from '@/components/common/HelpTooltip.vue'

defineProps<{
  label: string
  display: string
  amount: number
  countLabel: string
  hint: string
  testId: string
  rateTestId: string
  sticky?: boolean
}>()
</script>
