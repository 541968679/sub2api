<template>
  <div
    class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600"
    data-testid="newapi-wallet-fields"
  >
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.title') }}</label>
      <p class="input-hint">{{ t('admin.accounts.newapiWallet.hint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.userId') }}</label>
      <input
        :value="userId"
        type="text"
        inputmode="numeric"
        class="input font-mono"
        data-testid="newapi-wallet-user-id"
        :placeholder="t('admin.accounts.newapiWallet.userIdPlaceholder')"
        @input="emit('update:userId', ($event.target as HTMLInputElement).value)"
      />
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.newapiWallet.accessToken') }}</label>
      <input
        :value="accessToken"
        type="password"
        class="input font-mono"
        autocomplete="new-password"
        data-1p-ignore
        data-lpignore="true"
        data-bwignore="true"
        data-testid="newapi-wallet-access-token"
        :placeholder="
          hasSavedToken
            ? t('admin.accounts.newapiWallet.accessTokenKeep')
            : t('admin.accounts.newapiWallet.accessTokenPlaceholder')
        "
        @input="emit('update:accessToken', ($event.target as HTMLInputElement).value)"
      />
    </div>
    <button
      v-if="hasSavedToken || userId || accessToken"
      type="button"
      class="text-sm text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400"
      data-testid="newapi-wallet-clear"
      @click="emit('clear')"
    >
      {{ t('admin.accounts.newapiWallet.clear') }}
    </button>
    <p v-if="hasSavedToken || userId || accessToken" class="input-hint">
      {{ t('admin.accounts.newapiWallet.clearHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  userId: string
  accessToken: string
  hasSavedToken?: boolean
}>()

const emit = defineEmits<{
  'update:userId': [value: string]
  'update:accessToken': [value: string]
  clear: []
}>()

const { t } = useI18n()
</script>
