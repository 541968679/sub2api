<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.updateRefreshTokenTitle')"
    width="normal"
    @close="handleClose"
  >
    <div v-if="account" class="space-y-4">
      <!-- Account Info -->
      <div
        class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br',
              isOpenAI
                ? 'from-green-500 to-green-600'
                : isGemini
                  ? 'from-blue-500 to-blue-600'
                  : isAntigravity
                    ? 'from-purple-500 to-purple-600'
                    : 'from-orange-500 to-orange-600'
            ]"
          >
            <Icon name="key" size="md" class="text-white" />
          </div>
          <div>
            <span class="block font-semibold text-gray-900 dark:text-white">{{
              account.name
            }}</span>
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ platformLabel }}</span>
          </div>
        </div>
      </div>

      <p class="text-sm text-gray-600 dark:text-gray-400">
        {{ t('admin.accounts.updateRefreshTokenDesc') }}
      </p>

      <!-- Refresh token input -->
      <div>
        <label class="input-label">{{ t('admin.accounts.refreshToken') }}</label>
        <textarea
          v-model="refreshToken"
          rows="4"
          class="input font-mono text-xs"
          :placeholder="t('admin.accounts.updateRefreshTokenPlaceholder')"
        ></textarea>
      </div>

      <!-- OpenAI optional client_id -->
      <div v-if="isOpenAI">
        <label class="input-label">{{ t('admin.accounts.clientIdOptional') }}</label>
        <input
          v-model="clientId"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.clientIdPlaceholder')"
        />
      </div>

      <!-- Validate-before-save toggle -->
      <label class="flex cursor-pointer items-start gap-2">
        <input
          v-model="validate"
          type="checkbox"
          class="mt-0.5 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
        />
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.validateBeforeSave') }}
          <span class="block text-xs text-gray-500 dark:text-gray-400">{{
            t('admin.accounts.validateBeforeSaveHint')
          }}</span>
        </span>
      </label>
    </div>

    <template #footer>
      <div v-if="account" class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          :disabled="!canSubmit"
          class="btn btn-primary"
          @click="handleSubmit"
        >
          <svg
            v-if="loading"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ loading ? t('admin.accounts.oauth.verifying') : t('admin.accounts.updateRefreshToken') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  show: boolean
  account: Account | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const appStore = useAppStore()
const { t } = useI18n()

const refreshToken = ref('')
const clientId = ref('')
const validate = ref(true)
const loading = ref(false)

const isOpenAI = computed(() => props.account?.platform === 'openai')
const isGemini = computed(() => props.account?.platform === 'gemini')
const isAntigravity = computed(() => props.account?.platform === 'antigravity')

const platformLabel = computed(() => {
  if (isOpenAI.value) return t('admin.accounts.openaiAccount')
  if (isGemini.value) return t('admin.accounts.geminiAccount')
  if (isAntigravity.value) return t('admin.accounts.antigravityAccount')
  return t('admin.accounts.claudeCodeAccount')
})

const canSubmit = computed(() => refreshToken.value.trim().length > 0 && !loading.value)

const resetState = () => {
  refreshToken.value = ''
  clientId.value = ''
  validate.value = true
  loading.value = false
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) resetState()
  }
)

const handleClose = () => {
  emit('close')
}

const handleSubmit = async () => {
  if (!props.account) return
  const token = refreshToken.value.trim()
  if (!token) return

  loading.value = true
  try {
    const updatedAccount = await adminAPI.accounts.updateRefreshToken(props.account.id, token, {
      validate: validate.value,
      clientId: isOpenAI.value ? clientId.value.trim() || undefined : undefined
    })
    appStore.showSuccess(t('admin.accounts.updateRefreshTokenSuccess'))
    emit('updated', updatedAccount)
    handleClose()
  } catch (error: any) {
    appStore.showError(
      error?.response?.data?.detail ||
        error?.response?.data?.message ||
        t('admin.accounts.updateRefreshTokenFailed')
    )
  } finally {
    loading.value = false
  }
}
</script>
