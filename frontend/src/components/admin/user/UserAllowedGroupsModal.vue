<template>
  <BaseDialog :show="show" :title="t('admin.users.groupConfig')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-6">
      <!-- 用户信息头部 -->
      <div class="flex items-center gap-4 rounded-2xl bg-gradient-to-r from-primary-50 to-primary-100 p-5 dark:from-primary-900/30 dark:to-primary-800/20">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.users.groupConfigHint', { email: user.email }) }}</p>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-12">
        <svg class="h-10 w-10 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div v-else class="space-y-6">
        <!-- 专属分组区域（标准） -->
        <div v-if="exclusiveGroupConfigs.length > 0">
          <div class="mb-3 flex items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-purple-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.users.exclusiveGroups') }}</h4>
            <span class="text-xs text-gray-400">({{ exclusiveGroupConfigs.filter(c => c.isSelected).length }}/{{ exclusiveGroupConfigs.length }})</span>
          </div>
          <div class="grid gap-3">
            <div
              v-for="config in exclusiveGroupConfigs"
              :key="config.groupId"
              class="group relative overflow-hidden rounded-xl border-2 p-4 transition-all duration-200"
              :class="config.isSelected
                ? 'border-primary-400 bg-primary-50/50 shadow-sm dark:border-primary-500 dark:bg-primary-900/20'
                : 'border-gray-200 bg-white hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500'"
            >
              <div class="flex items-center gap-4">
                <div class="flex-shrink-0">
                  <label class="relative flex h-6 w-6 cursor-pointer items-center justify-center">
                    <input
                      type="checkbox"
                      :checked="config.isSelected"
                      @change="toggleExclusiveGroup(config.groupId)"
                      class="peer sr-only"
                    />
                    <div class="h-5 w-5 rounded-md border-2 border-gray-300 transition-all peer-checked:border-primary-500 peer-checked:bg-primary-500 dark:border-dark-500 peer-checked:dark:border-primary-500">
                      <svg v-if="config.isSelected" class="h-full w-full text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    </div>
                  </label>
                </div>

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                    <span class="inline-flex items-center rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700 dark:bg-purple-900/40 dark:text-purple-300">
                      {{ t('admin.groups.exclusive') }}
                    </span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      <PlatformIcon :platform="config.platform" size="xs" />
                      <span>{{ config.platform }}</span>
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-3">
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ t('admin.users.customRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.customRate ?? ''"
                      @input="updateCustomRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultRate)"
                      class="hide-spinner w-20 rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-amber-600 dark:text-amber-400">{{ t('admin.users.displayRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.displayRate ?? ''"
                      @input="updateDisplayRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="t('admin.users.displayRatePlaceholder')"
                      class="hide-spinner w-20 rounded-lg border border-amber-300 bg-amber-50 px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-amber-500 focus:outline-none focus:ring-2 focus:ring-amber-500/20 dark:border-amber-700 dark:bg-amber-900/20 dark:focus:border-amber-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 公开分组区域（标准） -->
        <div v-if="publicGroupConfigs.length > 0">
          <div class="mb-3 flex items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-green-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.users.publicGroups') }}</h4>
            <span class="text-xs text-gray-400">({{ publicGroupConfigs.length }})</span>
          </div>
          <div class="grid gap-3">
            <div
              v-for="config in publicGroupConfigs"
              :key="config.groupId"
              class="relative overflow-hidden rounded-xl border-2 border-green-200 bg-green-50/50 p-4 dark:border-green-800/50 dark:bg-green-900/10"
            >
              <div class="flex items-center gap-4">
                <div class="flex-shrink-0">
                  <div class="flex h-5 w-5 items-center justify-center rounded-md border-2 border-green-400 bg-green-500 dark:border-green-600 dark:bg-green-600">
                    <svg class="h-full w-full text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="3">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  </div>
                </div>

                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      <PlatformIcon :platform="config.platform" size="xs" />
                      <span>{{ config.platform }}</span>
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-3">
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ t('admin.users.customRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.customRate ?? ''"
                      @input="updateCustomRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultRate)"
                      class="hide-spinner w-20 rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-amber-600 dark:text-amber-400">{{ t('admin.users.displayRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.displayRate ?? ''"
                      @input="updateDisplayRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="t('admin.users.displayRatePlaceholder')"
                      class="hide-spinner w-20 rounded-lg border border-amber-300 bg-amber-50 px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-amber-500 focus:outline-none focus:ring-2 focus:ring-amber-500/20 dark:border-amber-700 dark:bg-amber-900/20 dark:focus:border-amber-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 订阅分组区域 -->
        <div>
          <div class="mb-3 flex items-center gap-2">
            <div class="h-1.5 w-1.5 rounded-full bg-blue-500"></div>
            <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.users.subscriptionGroups') }}</h4>
            <span class="text-xs text-gray-400">({{ subscriptionGroupConfigs.length }})</span>
          </div>
          <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.users.subscriptionGroupsHint') }}</p>

          <div v-if="subscriptionGroupConfigs.length === 0" class="rounded-xl border border-dashed border-gray-200 px-4 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
            {{ t('admin.users.noSubscriptionGroups') }}
          </div>

          <div v-else class="grid gap-3">
            <div
              v-for="config in subscriptionGroupConfigs"
              :key="config.groupId"
              class="relative overflow-hidden rounded-xl border-2 border-blue-200 bg-blue-50/40 p-4 dark:border-blue-800/50 dark:bg-blue-900/10"
            >
              <div class="flex items-center gap-4">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-base font-semibold text-gray-900 dark:text-white">{{ config.groupName }}</span>
                    <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
                      {{ t('admin.groups.subscription.subscription') }}
                    </span>
                    <span
                      v-if="config.subscriptionStatus"
                      class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="subscriptionStatusClass(config.subscriptionStatus)"
                    >
                      {{ t(`admin.subscriptions.status.${config.subscriptionStatus}`) }}
                    </span>
                  </div>
                  <div class="mt-1.5 flex items-center gap-3 text-sm">
                    <span class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400">
                      <PlatformIcon :platform="config.platform" size="xs" />
                      <span>{{ config.platform }}</span>
                    </span>
                    <span class="text-gray-300 dark:text-dark-500">•</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      {{ t('admin.users.defaultRate') }}: <span class="font-medium text-gray-700 dark:text-gray-300">{{ config.defaultRate }}x</span>
                    </span>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-3">
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-gray-600 dark:text-gray-400">{{ t('admin.users.customRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.customRate ?? ''"
                      @input="updateCustomRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="String(config.defaultRate)"
                      class="hide-spinner w-20 rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                    />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <label class="text-sm font-medium text-amber-600 dark:text-amber-400">{{ t('admin.users.displayRate') }}</label>
                    <input
                      type="number"
                      step="0.001"
                      min="0"
                      :value="config.displayRate ?? ''"
                      @input="updateDisplayRate(config.groupId, ($event.target as HTMLInputElement).value)"
                      :placeholder="t('admin.users.displayRatePlaceholder')"
                      class="hide-spinner w-20 rounded-lg border border-amber-300 bg-amber-50 px-2.5 py-1.5 text-sm font-medium transition-colors focus:border-amber-500 focus:outline-none focus:ring-2 focus:ring-amber-500/20 dark:border-amber-700 dark:bg-amber-900/20 dark:focus:border-amber-500"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 无分组提示 -->
        <div
          v-if="exclusiveGroupConfigs.length === 0 && publicGroupConfigs.length === 0 && subscriptionGroupConfigs.length === 0"
          class="flex flex-col items-center justify-center py-12 text-center"
        >
          <div class="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
            <svg class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <p class="text-gray-500 dark:text-gray-400">{{ t('common.noGroupsAvailable') }}</p>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" class="btn btn-secondary px-5">{{ t('common.cancel') }}</button>
        <button @click="handleSave" :disabled="submitting" class="btn btn-primary px-6">
          <svg v-if="submitting" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser, Group, GroupPlatform, UserSubscription } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

interface GroupRateConfig {
  groupId: number
  groupName: string
  platform: GroupPlatform
  kind: 'standard_exclusive' | 'standard_public' | 'subscription'
  isExclusive: boolean
  defaultRate: number
  customRate: number | null
  displayRate: number | null
  isSelected: boolean
  subscriptionStatus?: UserSubscription['status']
}

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()
const appStore = useAppStore()

const groupConfigs = ref<GroupRateConfig[]>([])
const originalGroupRates = ref<Record<number, number>>({})
const originalGroupDisplayRates = ref<Record<number, number>>({})
const loading = ref(false)
const submitting = ref(false)

const exclusiveGroupConfigs = computed(() =>
  groupConfigs.value.filter((c) => c.kind === 'standard_exclusive')
)
const publicGroupConfigs = computed(() =>
  groupConfigs.value.filter((c) => c.kind === 'standard_public')
)
const subscriptionGroupConfigs = computed(() =>
  groupConfigs.value.filter((c) => c.kind === 'subscription')
)

watch(
  () => props.show,
  (v) => {
    if (v && props.user) {
      load()
    }
  }
)

const subscriptionStatusClass = (status: string) => {
  if (status === 'active') return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
  if (status === 'expired') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
  if (status === 'suspended') return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
}

const load = async () => {
  loading.value = true
  try {
    const [groupsRes, subsRes] = await Promise.all([
      adminAPI.groups.list(1, 1000),
      adminAPI.subscriptions.list(1, 200, { user_id: props.user!.id }).catch(() => ({ items: [] as UserSubscription[] }))
    ])

    const allActive = groupsRes.items.filter((g) => g.status === 'active')
    const standardGroups = allActive.filter((g) => g.subscription_type === 'standard')
    const subscriptionTypeGroups = allActive.filter((g) => g.subscription_type === 'subscription')

    const userAllowedGroups = props.user?.allowed_groups || []
    const userGroupRates = props.user?.group_rates || {}
    const userGroupDisplayRates = props.user?.group_display_rates || {}
    originalGroupRates.value = { ...userGroupRates }
    originalGroupDisplayRates.value = { ...userGroupDisplayRates }

    // Best subscription status per group (prefer active > suspended > expired > revoked)
    const statusPriority: Record<string, number> = {
      active: 4,
      suspended: 3,
      expired: 2,
      revoked: 1
    }
    const subStatusByGroup = new Map<number, UserSubscription['status']>()
    for (const sub of subsRes.items || []) {
      const prev = subStatusByGroup.get(sub.group_id)
      if (!prev || (statusPriority[sub.status] || 0) > (statusPriority[prev] || 0)) {
        subStatusByGroup.set(sub.group_id, sub.status)
      }
    }

    // Subscription groups to show: has subscription row OR has rate override
    const subscriptionGroupIds = new Set<number>()
    for (const id of subStatusByGroup.keys()) subscriptionGroupIds.add(id)
    for (const g of subscriptionTypeGroups) {
      if (userGroupRates[g.id] !== undefined || userGroupDisplayRates[g.id] !== undefined) {
        subscriptionGroupIds.add(g.id)
      }
    }

    const toConfig = (
      g: Group,
      kind: GroupRateConfig['kind'],
      isSelected: boolean,
      subscriptionStatus?: UserSubscription['status']
    ): GroupRateConfig => ({
      groupId: g.id,
      groupName: g.name,
      platform: g.platform,
      kind,
      isExclusive: g.is_exclusive,
      defaultRate: g.rate_multiplier,
      customRate: userGroupRates[g.id] ?? null,
      displayRate: userGroupDisplayRates[g.id] ?? null,
      isSelected,
      subscriptionStatus
    })

    const configs: GroupRateConfig[] = []

    for (const g of standardGroups) {
      if (g.is_exclusive) {
        configs.push(toConfig(g, 'standard_exclusive', userAllowedGroups.includes(g.id)))
      } else {
        configs.push(toConfig(g, 'standard_public', true))
      }
    }

    for (const g of subscriptionTypeGroups) {
      if (!subscriptionGroupIds.has(g.id)) continue
      configs.push(toConfig(g, 'subscription', true, subStatusByGroup.get(g.id)))
    }

    groupConfigs.value = configs
  } catch (error) {
    console.error('Failed to load groups:', error)
  } finally {
    loading.value = false
  }
}

const toggleExclusiveGroup = (groupId: number) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config && config.kind === 'standard_exclusive') {
    config.isSelected = !config.isSelected
  }
}

const updateCustomRate = (groupId: number, value: string) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config) {
    if (value === '' || value === null || value === undefined) {
      config.customRate = null
    } else {
      const numValue = parseFloat(value)
      config.customRate = isNaN(numValue) ? null : numValue
    }
  }
}

const updateDisplayRate = (groupId: number, value: string) => {
  const config = groupConfigs.value.find((c) => c.groupId === groupId)
  if (config) {
    if (value === '' || value === null || value === undefined) {
      config.displayRate = null
    } else {
      const numValue = parseFloat(value)
      config.displayRate = isNaN(numValue) ? null : numValue
    }
  }
}

const handleSave = async () => {
  if (!props.user) return
  submitting.value = true

  try {
    // Only standard exclusive groups control allowed_groups
    const allowedGroups = groupConfigs.value
      .filter((c) => c.kind === 'standard_exclusive' && c.isSelected)
      .map((c) => c.groupId)

    const groupRatesFull: Record<number, import('@/types').UserGroupRateData | null> = {}
    for (const c of groupConfigs.value) {
      const hadOriginalRate = originalGroupRates.value[c.groupId] !== undefined
      const hadOriginalDisplayRate = originalGroupDisplayRates.value[c.groupId] !== undefined
      const hasAnyValue = c.customRate !== null || c.displayRate !== null

      if (hasAnyValue) {
        groupRatesFull[c.groupId] = {
          rate: c.customRate,
          display_rate: c.displayRate
        }
      } else if (hadOriginalRate || hadOriginalDisplayRate) {
        groupRatesFull[c.groupId] = null
      }
    }

    await adminAPI.users.update(props.user.id, {
      allowed_groups: allowedGroups,
      group_rates_full: Object.keys(groupRatesFull).length > 0 ? groupRatesFull : undefined
    })

    appStore.showSuccess(t('admin.users.groupConfigUpdated'))
    emit('success')
    emit('close')
  } catch (error) {
    console.error('Failed to update user group config:', error)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>
