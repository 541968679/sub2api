<template>
  <div class="space-y-6">
    <div class="card border-l-4 border-emerald-500 p-4">
      <p class="text-sm text-gray-700 dark:text-gray-200">
        {{ t('admin.redeemPage.hint') }}
      </p>
      <router-link
        to="/redeem"
        class="mt-2 inline-block text-sm font-medium text-primary-600 hover:underline dark:text-primary-400"
        target="_blank"
      >
        {{ t('admin.redeemPage.previewButton') }} →
      </router-link>
    </div>

    <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <section class="card p-6">
        <label class="input-label">{{ t('admin.redeemPage.noticeLabel') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.redeemPage.noticeHint') }}
        </p>
        <textarea
          v-model="form.notice"
          rows="8"
          class="input font-mono text-sm"
          :placeholder="t('admin.redeemPage.noticePlaceholder')"
        ></textarea>
        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.redeemPage.clearHint') }}
        </p>
      </section>

      <div v-if="form.notice.trim()" class="card p-4">
        <p class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
          {{ t('admin.redeemPage.previewLabel') }}
        </p>
        <SoftNoticeBanner
          :title="t('redeem.buyNoticeTitle')"
          :text="form.notice.trim()"
        />
      </div>

      <div class="flex items-center justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClear">
          {{ t('admin.redeemPage.clearButton') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="handleSave">
          {{ saving ? t('common.saving') : t('admin.redeemPage.saveButton') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemPageAPI } from '@/api/redeemPage'
import { useAppStore } from '@/stores'
import SoftNoticeBanner from '@/components/common/SoftNoticeBanner.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const form = reactive({ notice: '' })

async function load() {
  loading.value = true
  try {
    const data = await redeemPageAPI.getAdminRedeemPageContent()
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
    await redeemPageAPI.updateAdminRedeemPageContent({ notice: form.notice.trim() })
    form.notice = form.notice.trim()
    appStore.showSuccess(t('admin.redeemPage.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    saving.value = false
  }
}

async function handleClear() {
  if (!window.confirm(t('admin.redeemPage.clearConfirm'))) return
  form.notice = ''
  await handleSave()
}

onMounted(load)
</script>
