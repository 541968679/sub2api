<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-6xl">
      <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.errorPolicy.title') }}
      </h1>
      <div class="mt-4">
        <ScheduleErrorWhitelistPanel />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScheduleErrorWhitelistPanel from '@/components/admin/usage/ScheduleErrorWhitelistPanel.vue'

const { t } = useI18n()
</script>
