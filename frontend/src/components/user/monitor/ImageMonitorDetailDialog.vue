<template>
  <BaseDialog :show="show" :title="title" @close="emit('close')">
    <div v-if="loading" class="py-10 text-center text-sm text-gray-400 dark:text-dark-500">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="detail" class="space-y-3">
      <div class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('channelStatus.imageSection.detailModel') }}:
        <span class="font-mono">{{ detail.model }}</span>
      </div>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs uppercase text-gray-500 dark:text-dark-400">
            <th class="py-2 pr-4">{{ t('channelStatus.imageSection.detailWindow') }}</th>
            <th class="py-2 pr-4">{{ t('channelStatus.imageSection.detailAvailability') }}</th>
            <th class="py-2">{{ t('channelStatus.imageSection.detailAvgLatency') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-for="w in detail.windows" :key="w.window_days">
            <td class="py-2.5 pr-4">{{ t(`channelStatus.windowTab.${w.window_days}d`) }}</td>
            <td class="py-2.5 pr-4 tabular-nums font-medium">{{ w.availability.toFixed(1) }}%</td>
            <td class="py-2.5 tabular-nums">
              {{ w.avg_api_total_ms != null ? `${w.avg_api_total_ms.toLocaleString()} ms` : '-' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <template #footer>
      <button type="button" class="btn btn-primary" @click="emit('close')">
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  status as fetchImageMonitorDetail,
  type ImageMonitorPublicDetail,
} from '@/api/imageChannelMonitor'

const props = defineProps<{
  show: boolean
  monitorId: number | null
  title: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()
const detail = ref<ImageMonitorPublicDetail | null>(null)
const loading = ref(false)

watch(
  () => [props.show, props.monitorId] as const,
  async ([show, id]) => {
    if (!show || typeof id !== 'number') return
    loading.value = true
    detail.value = null
    try {
      detail.value = await fetchImageMonitorDetail(id)
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
    } finally {
      loading.value = false
    }
  }
)
</script>
