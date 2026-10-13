<template>
  <AppLayout>
    <div class="mx-auto flex min-h-0 max-w-3xl flex-col px-4 py-6 md:px-6">
      <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('admin.errorPolicy.title') }}
      </h1>
      <div class="card mt-4 flex min-h-[32rem] flex-col p-4">
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
