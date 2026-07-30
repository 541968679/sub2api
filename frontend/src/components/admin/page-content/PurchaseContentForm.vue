<template>
  <div class="space-y-6">
    <div class="card border-l-4 border-amber-500 p-4">
      <p class="text-sm text-gray-700 dark:text-gray-200">
        {{ t('admin.purchasePage.hint') }}
      </p>
      <router-link
        to="/purchase"
        class="mt-2 inline-block text-sm font-medium text-primary-600 hover:underline dark:text-primary-400"
        target="_blank"
      >
        {{ t('admin.purchasePage.previewButton') }} →
      </router-link>
    </div>

    <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <section class="card p-6">
        <label class="input-label">{{ t('admin.purchasePage.noticeLabel') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.purchasePage.noticeHint') }}
        </p>
        <textarea
          v-model="form.notice"
          rows="8"
          class="input font-mono text-sm"
          :placeholder="t('admin.purchasePage.noticePlaceholder')"
        ></textarea>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.purchasePage.clearHint') }}
        </p>
      </section>

      <div v-if="form.notice.trim()" class="card p-4">
        <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
          {{ t('admin.purchasePage.previewLabel') }}
        </p>
        <div
          class="flex items-start gap-3 rounded-xl border-2 border-red-400 bg-red-50 px-4 py-4 text-red-900 shadow-sm dark:border-red-500/70 dark:bg-red-950/40 dark:text-red-100"
          role="status"
        >
          <span class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-red-500 text-sm font-bold text-white">
            !
          </span>
          <p class="min-w-0 flex-1 whitespace-pre-line text-base font-semibold leading-7">
            {{ form.notice.trim() }}
          </p>
        </div>
      </div>

      <div class="flex items-center justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClear">
          {{ t('admin.purchasePage.clearButton') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="handleSave">
          {{ saving ? t('common.saving') : t('admin.purchasePage.saveButton') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { purchasePageAPI } from '@/api/purchasePage'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const form = reactive({ notice: '' })

async function load() {
  loading.value = true
  try {
    const data = await purchasePageAPI.getAdminPurchasePageContent()
    form.notice = data.notice ?? ''
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await purchasePageAPI.updateAdminPurchasePageContent({ notice: form.notice.trim() })
    form.notice = form.notice.trim()
    appStore.showSuccess(t('admin.purchasePage.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    saving.value = false
  }
}

async function handleClear() {
  if (!window.confirm(t('admin.purchasePage.clearConfirm'))) return
  form.notice = ''
  await handleSave()
}

onMounted(load)
</script>
