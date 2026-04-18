<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-4xl space-y-6">
      <!-- Header -->
      <div class="flex items-start justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.loginPage.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.loginPage.description') }}
          </p>
        </div>
        <router-link to="/login" target="_blank" class="btn btn-secondary text-sm shrink-0">
          {{ t('admin.loginPage.preview') }}
        </router-link>
      </div>

      <div class="card border-l-4 border-primary-500 p-4 text-sm text-gray-700 dark:text-gray-200">
        {{ t('admin.loginPage.fallbackHint') }}
      </div>

      <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <!-- 左栏营销文案 -->
        <section class="card p-6 space-y-4">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.loginPage.sections.marketing') }}
          </h2>

          <div>
            <label class="input-label">{{ t('admin.loginPage.fields.badge') }}</label>
            <input
              v-model="form.badge"
              type="text"
              class="input text-sm"
              :maxlength="255"
              :placeholder="t('auth.login.badge')"
            />
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.loginPage.fields.headingLine1') }}</label>
              <input
                v-model="form.heading_line1"
                type="text"
                class="input text-sm"
                :maxlength="255"
                :placeholder="t('auth.login.headingLine1')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.loginPage.fields.headingLine2') }}</label>
              <input
                v-model="form.heading_line2"
                type="text"
                class="input text-sm"
                :maxlength="255"
                :placeholder="t('auth.login.headingLine2')"
              />
            </div>
          </div>

          <div>
            <label class="input-label">{{ t('admin.loginPage.fields.description') }}</label>
            <textarea
              v-model="form.description"
              rows="3"
              class="input text-sm"
              :maxlength="500"
              :placeholder="t('auth.login.description')"
            ></textarea>
          </div>
        </section>

        <!-- 模型区文案 -->
        <section class="card p-6 space-y-4">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.loginPage.sections.models') }}
          </h2>

          <div>
            <label class="input-label">{{ t('admin.loginPage.fields.supportedModelsTitle') }}</label>
            <input
              v-model="form.supported_models_title"
              type="text"
              class="input text-sm"
              :maxlength="255"
              :placeholder="t('auth.login.supportedModels')"
            />
          </div>

          <div>
            <label class="input-label">{{ t('admin.loginPage.fields.modelsDesc') }}</label>
            <textarea
              v-model="form.models_desc"
              rows="2"
              class="input text-sm"
              :maxlength="500"
              :placeholder="t('auth.login.modelsDesc')"
            ></textarea>
          </div>
        </section>

        <!-- 右栏登录框 -->
        <section class="card p-6 space-y-4">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('admin.loginPage.sections.form') }}
          </h2>

          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.loginPage.fields.formTitle') }}</label>
              <input
                v-model="form.form_title"
                type="text"
                class="input text-sm"
                :maxlength="255"
                :placeholder="t('auth.login.title')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.loginPage.fields.formSubtitle') }}</label>
              <input
                v-model="form.form_subtitle"
                type="text"
                class="input text-sm"
                :maxlength="255"
                :placeholder="t('auth.login.subtitle')"
              />
            </div>
          </div>
        </section>

        <div class="flex items-center justify-between gap-3">
          <button
            type="button"
            class="btn btn-secondary text-sm"
            :disabled="saving || resetting"
            @click="handleReset"
          >
            {{ resetting ? t('common.saving') : t('admin.loginPage.resetButton') }}
          </button>

          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving || resetting"
            @click="handleSave"
          >
            {{ saving ? t('common.saving') : t('admin.loginPage.saveButton') }}
          </button>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { loginPageAPI, type LoginPageContent } from '@/api/loginPage'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const resetting = ref(false)

const form = reactive<LoginPageContent>({
  badge: '',
  heading_line1: '',
  heading_line2: '',
  description: '',
  supported_models_title: '',
  models_desc: '',
  form_title: '',
  form_subtitle: ''
})

function applyToForm(payload: LoginPageContent) {
  form.badge = payload.badge ?? ''
  form.heading_line1 = payload.heading_line1 ?? ''
  form.heading_line2 = payload.heading_line2 ?? ''
  form.description = payload.description ?? ''
  form.supported_models_title = payload.supported_models_title ?? ''
  form.models_desc = payload.models_desc ?? ''
  form.form_title = payload.form_title ?? ''
  form.form_subtitle = payload.form_subtitle ?? ''
}

async function load() {
  loading.value = true
  try {
    const data = await loginPageAPI.getAdminLoginPageContent()
    applyToForm(data)
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const saved = await loginPageAPI.updateAdminLoginPageContent({ ...form })
    applyToForm(saved)
    appStore.showSuccess(t('admin.loginPage.saveSuccess'))
    // 让下次从登录页进来立刻看到新内容
    await appStore.fetchPublicSettings(true)
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    saving.value = false
  }
}

async function handleReset() {
  if (!window.confirm(t('admin.loginPage.resetConfirm'))) return
  resetting.value = true
  try {
    const cleared = await loginPageAPI.resetAdminLoginPageContent()
    applyToForm(cleared)
    appStore.showSuccess(t('admin.loginPage.resetSuccess'))
    await appStore.fetchPublicSettings(true)
  } catch (err: unknown) {
    appStore.showError(err instanceof Error ? err.message : String(err))
  } finally {
    resetting.value = false
  }
}

onMounted(load)
</script>
