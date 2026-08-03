<template>
  <div>
    <button
      class="relative flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-all hover:scale-105 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-800"
      :class="{ 'text-red-600 dark:text-red-400': unreadCount > 0 }"
      :aria-label="t('admin.banNotifications.title')"
      @click="openModal"
    >
      <Icon name="shield" size="md" />
      <span
        v-if="unreadCount > 0"
        class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white"
      >
        {{ unreadCount > 99 ? '99+' : unreadCount }}
      </span>
    </button>

    <Teleport to="body">
      <Transition name="modal-fade">
        <div
          v-if="isModalOpen"
          class="fixed inset-0 z-[100] flex items-start justify-center overflow-y-auto bg-gradient-to-br from-black/70 via-black/60 to-black/70 p-4 pt-[8vh] backdrop-blur-md"
          @click="closeModal"
        >
          <div
            class="w-full max-w-[640px] overflow-hidden rounded-3xl bg-white shadow-2xl ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
            @click.stop
          >
            <div class="relative overflow-hidden border-b border-gray-100/80 bg-gradient-to-br from-red-50/60 to-orange-50/30 px-6 py-5 dark:border-dark-700/50 dark:from-red-900/15 dark:to-orange-900/5">
              <div class="relative z-10 flex items-start justify-between gap-3">
                <div>
                  <div class="flex items-center gap-2">
                    <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-red-500 to-orange-600 text-white shadow-lg shadow-red-500/30">
                      <Icon name="shield" size="sm" />
                    </div>
                    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.banNotifications.title') }}
                    </h2>
                  </div>
                  <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
                    <template v-if="unreadCount > 0">
                      <span class="font-medium text-red-600 dark:text-red-400">{{ unreadCount }}</span>
                      {{ t('admin.banNotifications.unread') }}
                    </template>
                    <template v-else>
                      {{ t('admin.banNotifications.noUnread') }}
                    </template>
                  </p>
                </div>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    v-if="unreadCount > 0"
                    class="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white shadow-lg shadow-red-500/20 transition hover:bg-red-700 disabled:opacity-50"
                    :disabled="actionLoading"
                    @click="onMarkAllRead"
                  >
                    {{ t('admin.banNotifications.markAllRead') }}
                  </button>
                  <button
                    v-if="items.length > 0"
                    class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition hover:bg-gray-50 disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
                    :disabled="actionLoading"
                    @click="onClearAll"
                  >
                    {{ t('admin.banNotifications.clearAll') }}
                  </button>
                  <button
                    class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/50 text-gray-500 backdrop-blur-sm transition hover:bg-white hover:text-gray-700 dark:bg-dark-700/50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                    :aria-label="t('common.close')"
                    @click="closeModal"
                  >
                    <Icon name="x" size="sm" />
                  </button>
                </div>
              </div>
            </div>

            <div class="max-h-[65vh] overflow-y-auto">
              <div v-if="loading && items.length === 0" class="flex items-center justify-center py-16">
                <div class="h-10 w-10 animate-spin rounded-full border-4 border-gray-200 border-t-red-600 dark:border-dark-600 dark:border-t-red-400" />
              </div>

              <div v-else-if="items.length > 0">
                <div
                  v-for="item in items"
                  :key="item.id"
                  class="group relative border-b border-gray-100 px-6 py-4 transition hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/30"
                  :class="{ 'bg-red-50/30 dark:bg-red-900/5': !item.read }"
                >
                  <div
                    v-if="!item.read"
                    class="absolute left-0 top-0 h-full w-1 bg-gradient-to-b from-red-500 to-orange-500"
                  />
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0 flex-1">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                          {{ item.user_email || `UID ${item.user_id}` }}
                        </span>
                        <span
                          class="rounded-md px-1.5 py-0.5 text-[11px] font-medium"
                          :class="statusClass(item.user_status)"
                        >
                          {{ statusLabel(item.user_status) }}
                        </span>
                        <span
                          v-if="!item.read"
                          class="rounded-md bg-red-100 px-1.5 py-0.5 text-[11px] font-medium text-red-700 dark:bg-red-900/40 dark:text-red-300"
                        >
                          {{ t('admin.banNotifications.unreadBadge') }}
                        </span>
                      </div>
                      <p class="mt-1 text-xs text-gray-600 dark:text-gray-400">
                        {{ t('admin.banNotifications.summary', {
                          category: item.highest_category || '-',
                          count: item.violation_count,
                          threshold: item.ban_threshold,
                        }) }}
                      </p>
                      <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-gray-500 dark:text-gray-500">
                        <span>{{ formatTime(item.created_at) }}</span>
                        <span v-if="item.group_name">{{ item.group_name }}</span>
                        <span v-if="item.model">{{ item.model }}</span>
                        <span>UID {{ item.user_id }}</span>
                      </div>
                    </div>
                    <div class="flex flex-shrink-0 flex-col gap-1.5">
                      <button
                        v-if="item.user_status === 'disabled'"
                        class="rounded-lg bg-emerald-600 px-2.5 py-1 text-[11px] font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50"
                        :disabled="actionLoading"
                        @click="onUnban(item)"
                      >
                        {{ t('admin.banNotifications.quickUnban') }}
                      </button>
                      <button
                        v-if="!item.read"
                        class="rounded-lg border border-gray-200 px-2.5 py-1 text-[11px] font-medium text-gray-600 transition hover:bg-gray-50 disabled:opacity-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700"
                        :disabled="actionLoading"
                        @click="onMarkRead(item)"
                      >
                        {{ t('admin.banNotifications.markRead') }}
                      </button>
                      <button
                        class="rounded-lg border border-gray-200 px-2.5 py-1 text-[11px] font-medium text-gray-500 transition hover:bg-gray-50 disabled:opacity-50 dark:border-dark-600 dark:text-gray-400 dark:hover:bg-dark-700"
                        :disabled="actionLoading"
                        @click="onClearOne(item)"
                      >
                        {{ t('admin.banNotifications.clearOne') }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              <div v-else class="flex flex-col items-center justify-center py-16">
                <div class="mb-3 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
                  <Icon name="shield" size="lg" class="text-gray-400" />
                </div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('admin.banNotifications.empty') }}
                </p>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.banNotifications.emptyHint') }}
                </p>
              </div>
            </div>

            <div class="border-t border-gray-100 px-6 py-3 dark:border-dark-700">
              <router-link
                to="/admin/risk-control"
                class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                @click="closeModal"
              >
                {{ t('admin.banNotifications.openRiskControl') }}
              </router-link>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBanNotificationStore } from '@/stores/banNotifications'
import { useAppStore } from '@/stores'
import type { AdminRiskBanNotification } from '@/api/admin/riskControl'

const { t, locale } = useI18n()
const store = useBanNotificationStore()
const appStore = useAppStore()

const isModalOpen = ref(false)
const actionLoading = ref(false)

const items = computed(() => store.items)
const unreadCount = computed(() => store.unreadCount)
const loading = computed(() => store.loading)

async function openModal() {
  isModalOpen.value = true
  await store.fetchNotifications(true)
}

function closeModal() {
  isModalOpen.value = false
}

function formatTime(value: string) {
  if (!value) return ''
  try {
    return new Date(value).toLocaleString(locale.value === 'zh' ? 'zh-CN' : 'en-US')
  } catch {
    return value
  }
}

function statusLabel(status: string) {
  if (status === 'disabled') return t('admin.banNotifications.statusDisabled')
  if (status === 'active') return t('admin.banNotifications.statusActive')
  return status || t('admin.banNotifications.statusUnknown')
}

function statusClass(status: string) {
  if (status === 'disabled') {
    return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
  }
  if (status === 'active') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
  }
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

async function onMarkAllRead() {
  actionLoading.value = true
  try {
    await store.markRead(undefined, true)
    appStore.showSuccess(t('admin.banNotifications.markAllReadSuccess'))
  } catch {
    appStore.showError(t('admin.banNotifications.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

async function onMarkRead(item: AdminRiskBanNotification) {
  actionLoading.value = true
  try {
    await store.markRead([item.id])
  } catch {
    appStore.showError(t('admin.banNotifications.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

async function onClearAll() {
  if (!window.confirm(t('admin.banNotifications.clearAllConfirm'))) return
  actionLoading.value = true
  try {
    await store.clear(undefined, true)
    appStore.showSuccess(t('admin.banNotifications.clearSuccess'))
  } catch {
    appStore.showError(t('admin.banNotifications.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

async function onClearOne(item: AdminRiskBanNotification) {
  actionLoading.value = true
  try {
    await store.clear([item.id], false)
  } catch {
    appStore.showError(t('admin.banNotifications.actionFailed'))
  } finally {
    actionLoading.value = false
  }
}

async function onUnban(item: AdminRiskBanNotification) {
  if (!window.confirm(t('admin.banNotifications.unbanConfirm', { email: item.user_email || item.user_id }))) {
    return
  }
  actionLoading.value = true
  try {
    await store.quickUnban(item.user_id)
    if (!item.read) {
      await store.markRead([item.id])
    }
    appStore.showSuccess(t('admin.riskControl.unbanSuccess'))
  } catch {
    appStore.showError(t('admin.riskControl.unbanFailed'))
  } finally {
    actionLoading.value = false
  }
}
</script>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
