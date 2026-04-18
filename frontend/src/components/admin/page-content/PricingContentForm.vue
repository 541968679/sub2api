<template>
  <div class="space-y-6">
    <!-- Hint: model visibility lives in model config -->
    <div class="card border-l-4 border-primary-500 p-4">
      <p class="text-sm text-gray-700 dark:text-gray-200">
        {{ t('admin.pricingPage.modelSelectHint') }}
      </p>
      <router-link
        to="/admin/model-config"
        class="mt-2 inline-block text-sm font-medium text-primary-600 hover:underline dark:text-primary-400"
      >
        {{ t('admin.pricingPage.modelConfigLink') }} →
      </router-link>
    </div>

    <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <section class="card p-6">
        <label class="input-label">{{ t('admin.pricingPage.introLabel') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.pricingPage.introHint') }}
        </p>
        <textarea
          v-model="form.intro"
          rows="10"
          class="input font-mono text-sm"
          :placeholder="t('admin.pricingPage.introPlaceholder')"
        ></textarea>
      </section>

      <section class="card p-6">
        <label class="input-label">{{ t('admin.pricingPage.educationLabel') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.pricingPage.educationHint') }}
        </p>
        <textarea
          v-model="form.education"
          rows="14"
          class="input font-mono text-sm"
          :placeholder="t('admin.pricingPage.educationPlaceholder')"
        ></textarea>
      </section>

      <div class="flex items-center justify-end gap-3">
        <router-link to="/pricing" class="btn btn-secondary" target="_blank">
          {{ t('admin.pricingPage.previewButton') }}
        </router-link>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="handleSave">
          {{ saving ? t('common.saving') : t('admin.pricingPage.saveButton') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { pricingPageAPI } from '@/api/pricingPage'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const form = reactive({ intro: '', education: '' })

async function load() {
  loading.value = true
  try {
    const data = await pricingPageAPI.getAdminPricingPageContent()
    form.intro = data.intro ?? ''
    form.education = data.education ?? ''
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await pricingPageAPI.updateAdminPricingPageContent({ intro: form.intro, education: form.education })
    appStore.showSuccess(t('admin.pricingPage.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
