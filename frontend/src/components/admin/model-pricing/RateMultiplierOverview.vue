<template>
  <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
    <button
      @click="expanded = !expanded"
      class="flex w-full items-center justify-between p-4 text-left"
    >
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.modelPricing.rateMultipliers') }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.modelPricing.rateMultipliersHint') }}
        </p>
      </div>
      <svg
        class="h-4 w-4 text-gray-400 transition-transform"
        :class="expanded ? 'rotate-180' : ''"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        stroke-width="2"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div v-if="expanded" class="border-t border-gray-200 dark:border-gray-700">
      <div v-if="loading" class="flex items-center justify-center py-8">
        <span class="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></span>
      </div>
      <div v-else-if="items.length === 0" class="py-6 text-center text-sm text-gray-400">
        {{ t('admin.modelPricing.noGroups') }}
      </div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="border-b border-gray-100 dark:border-gray-700">
            <th class="px-4 py-2 text-left font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.modelPricing.groupName') }}
            </th>
            <th class="px-4 py-2 text-right font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.modelPricing.rateMultiplier') }}
            </th>
            <th class="px-4 py-2 text-right font-medium text-gray-500 dark:text-gray-400"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in items"
            :key="item.group_id"
            class="border-b border-gray-50 last:border-0 dark:border-gray-700/50"
          >
            <td class="px-4 py-2 text-gray-900 dark:text-white">{{ item.group_name }}</td>
            <td class="px-4 py-2 text-right font-mono text-gray-700 dark:text-gray-300">
              <span :class="item.rate_multiplier !== 1.0 ? 'text-amber-600 dark:text-amber-400 font-semibold' : ''">
                {{ item.rate_multiplier.toFixed(2) }}x
              </span>
            </td>
            <td class="px-4 py-2 text-right">
              <router-link
                :to="`/admin/groups`"
                class="text-xs text-primary-600 hover:underline dark:text-primary-400"
              >
                {{ t('admin.modelPricing.edit') }}
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { RateMultiplierSummary } from '@/api/admin/modelPricing'

const { t } = useI18n()
const appStore = useAppStore()

const expanded = ref(false)
const loading = ref(false)
const items = ref<RateMultiplierSummary[]>([])

async function loadData() {
  loading.value = true
  try {
    items.value = await adminAPI.modelPricing.getRateMultiplierOverview()
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>
