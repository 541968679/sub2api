<template>
  <div class="mx-auto max-w-6xl space-y-6 p-4 md:p-6">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
        {{ t('modelPlaza.title') }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ description || t('modelPlaza.description') }}
      </p>
    </div>

    <div v-if="loading" class="text-sm text-gray-500">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="text-sm text-red-600">{{ error }}</div>
    <div v-else-if="groups.length === 0" class="text-sm text-gray-500">
      {{ t('modelPlaza.empty') }}
    </div>

    <section
      v-for="group in groups"
      :key="group.id"
      class="overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
    >
      <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ group.name }}</h2>
          <p v-if="group.description" class="text-xs text-gray-500">{{ group.description }}</p>
        </div>
        <div class="text-right text-xs text-gray-500">
          <div>{{ group.platform }}</div>
          <div>{{ t('modelPlaza.rate') }} ×{{ group.rate_multiplier }}</div>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="min-w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-2">{{ t('modelPlaza.columns.model') }}</th>
              <th class="px-4 py-2">{{ t('modelPlaza.columns.displayInput') }}</th>
              <th class="px-4 py-2">{{ t('modelPlaza.columns.displayOutput') }}</th>
              <th class="px-4 py-2">{{ t('modelPlaza.columns.displayCache') }}</th>
              <th class="px-4 py-2">{{ t('modelPlaza.columns.officialInput') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="model in group.models"
              :key="`${group.id}-${model.platform}-${model.name}`"
              class="border-t border-gray-100 dark:border-dark-700"
            >
              <td class="px-4 py-2 font-medium text-gray-900 dark:text-white">{{ model.name }}</td>
              <td class="px-4 py-2">{{ formatPrice(model.pricing?.input_price) }}</td>
              <td class="px-4 py-2">{{ formatPrice(model.pricing?.output_price) }}</td>
              <td class="px-4 py-2">{{ formatPrice(model.pricing?.cache_read_price) }}</td>
              <td class="px-4 py-2 text-gray-500">{{ formatPrice(model.official_pricing?.input_price) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getModelPlaza, type ModelPlazaGroup } from '@/api/modelPlaza'

const { t } = useI18n()
const loading = ref(true)
const error = ref('')
const description = ref('')
const groups = ref<ModelPlazaGroup[]>([])

function formatPrice(value?: number | null): string {
  if (value == null) return '—'
  return `$${(value * 1_000_000).toFixed(2)} / MTok`
}

onMounted(async () => {
  try {
    const data = await getModelPlaza()
    description.value = data.description || ''
    groups.value = data.groups || []
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('modelPlaza.loadFailed')
  } finally {
    loading.value = false
  }
})
</script>
