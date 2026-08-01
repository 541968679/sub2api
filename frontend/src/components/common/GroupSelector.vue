<template>
  <div :class="variant === 'panel' ? 'flex h-full min-h-0 flex-col' : ''">
    <div class="mb-1.5 flex items-center justify-between gap-3">
      <label class="input-label mb-0">
        {{ t('admin.users.groups') }}
        <span class="font-normal text-gray-400">{{ t('common.selectedCount', { count: modelValue.length }) }}</span>
      </label>
      <button
        v-if="showToggleAll"
        type="button"
        class="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-600 transition-colors hover:border-primary-300 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:border-primary-700 dark:hover:text-primary-400"
        :disabled="filteredGroups.length === 0"
        data-testid="group-selector-toggle-all"
        @click="toggleAllGroups"
      >
        <Icon :name="allFilteredGroupsSelected ? 'x' : 'check'" size="xs" :stroke-width="2" />
        {{ allFilteredGroupsSelected ? t('common.deselectAll') : t('common.selectAll') }}
      </button>
    </div>

    <!-- Platform / subscription-type quick filters -->
    <div v-if="showQuickFilters && filteredGroups.length > 0" class="mb-2 space-y-2">
      <div v-if="availablePlatforms.length > 0" class="flex flex-wrap items-center gap-1.5">
        <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">
          {{ t('common.groupSelector.byPlatform') }}
        </span>
        <button
          v-for="p in availablePlatforms"
          :key="p"
          type="button"
          :data-testid="`group-selector-platform-${p}`"
          :class="[
            'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-medium transition-colors',
            isPlatformFullySelected(p)
              ? 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
              : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:border-primary-700'
          ]"
          @click="togglePlatform(p)"
        >
          <PlatformIcon :platform="p" size="xs" />
          {{ platformLabel(p) }}
          <span class="opacity-60">({{ platformGroupIds(p).length }})</span>
        </button>
      </div>
      <div class="flex flex-wrap items-center gap-1.5">
        <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">
          {{ t('common.groupSelector.byType') }}
        </span>
        <button
          v-for="st in availableSubscriptionTypes"
          :key="st"
          type="button"
          :data-testid="`group-selector-type-${st}`"
          :class="[
            'inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium transition-colors',
            isSubscriptionTypeFullySelected(st)
              ? 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-600 dark:bg-primary-900/30 dark:text-primary-300'
              : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:border-primary-700'
          ]"
          @click="toggleSubscriptionType(st)"
        >
          {{ subscriptionTypeLabel(st) }}
          <span class="ml-1 opacity-60">({{ subscriptionTypeGroupIds(st).length }})</span>
        </button>
      </div>
    </div>

    <div
      :class="[
        'grid gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-800',
        variant === 'panel'
          ? 'min-h-[140px] max-h-[min(42vh,360px)] flex-1 grid-cols-1 content-start sm:min-h-[180px] lg:min-h-[220px] lg:max-h-none'
          : 'max-h-32 grid-cols-2'
      ]"
    >
      <label
        v-for="group in filteredGroups"
        :key="group.id"
        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 transition-colors hover:bg-white dark:hover:bg-dark-700"
        :title="t('admin.groups.rateAndAccounts', { rate: group.rate_multiplier, count: group.account_count || 0 })"
      >
        <input
          type="checkbox"
          :value="group.id"
          :checked="modelValue.includes(group.id)"
          @change="handleChange(group.id, ($event.target as HTMLInputElement).checked)"
          class="h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
        />
        <GroupBadge
          :name="group.name"
          :platform="group.platform"
          :subscription-type="group.subscription_type"
          :rate-multiplier="group.rate_multiplier"
          class="min-w-0 flex-1"
        />
        <span class="shrink-0 text-xs text-gray-400">{{ group.account_count || 0 }}</span>
      </label>
      <div
        v-if="filteredGroups.length === 0"
        class="col-span-full py-2 text-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('common.noGroupsAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup, GroupPlatform, SubscriptionType } from '@/types'

const { t } = useI18n()

interface Props {
  modelValue: number[]
  groups: AdminGroup[]
  platform?: GroupPlatform // Optional platform filter
  mixedScheduling?: boolean // For antigravity accounts: allow anthropic/gemini groups
  extraPlatforms?: GroupPlatform[] // Extra platforms allowed by account-side bridge capabilities
  showToggleAll?: boolean
  /** Show one-click select chips by platform and subscription type */
  showQuickFilters?: boolean
  /** panel = taller list for account edit multi-column layout */
  variant?: 'default' | 'panel'
}

const props = withDefaults(defineProps<Props>(), {
  showToggleAll: false,
  showQuickFilters: false,
  variant: 'default'
})
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

// Filter groups by platform if specified
const filteredGroups = computed(() => {
  if (!props.platform) {
    return props.groups
  }
  const allowedPlatforms = new Set<GroupPlatform>([props.platform, ...(props.extraPlatforms || [])])
  // antigravity 账户启用混合调度后，可选择 anthropic/gemini 分组
  if (props.platform === 'antigravity' && props.mixedScheduling) {
    allowedPlatforms.add('anthropic')
    allowedPlatforms.add('gemini')
  }
  // 默认只能选择同 platform 分组；账号侧 bridge 能力可额外放开指定平台。
  return props.groups.filter((g) => allowedPlatforms.has(g.platform))
})

const filteredGroupIds = computed(() => filteredGroups.value.map((group) => group.id))

const allFilteredGroupsSelected = computed(
  () =>
    filteredGroupIds.value.length > 0 &&
    filteredGroupIds.value.every((groupId) => props.modelValue.includes(groupId))
)

const availablePlatforms = computed(() => {
  const seen = new Set<GroupPlatform>()
  const order: GroupPlatform[] = []
  for (const g of filteredGroups.value) {
    if (!seen.has(g.platform)) {
      seen.add(g.platform)
      order.push(g.platform)
    }
  }
  return order
})

const availableSubscriptionTypes = computed(() => {
  const seen = new Set<SubscriptionType>()
  const order: SubscriptionType[] = []
  for (const g of filteredGroups.value) {
    const st = (g.subscription_type || 'standard') as SubscriptionType
    if (!seen.has(st)) {
      seen.add(st)
      order.push(st)
    }
  }
  return order
})

const PLATFORM_I18N: Record<GroupPlatform, string> = {
  anthropic: 'admin.groups.platforms.anthropic',
  openai: 'admin.groups.platforms.openai',
  gemini: 'admin.groups.platforms.gemini',
  antigravity: 'admin.groups.platforms.antigravity',
  grok: 'admin.groups.platforms.grok'
}

const platformLabel = (platform: GroupPlatform) => t(PLATFORM_I18N[platform] || platform)

const subscriptionTypeLabel = (st: SubscriptionType) => {
  if (st === 'subscription') {
    return t('admin.groups.subscription.subscription')
  }
  return t('admin.groups.subscription.standard')
}

const platformGroupIds = (platform: GroupPlatform) =>
  filteredGroups.value.filter((g) => g.platform === platform).map((g) => g.id)

const subscriptionTypeGroupIds = (st: SubscriptionType) =>
  filteredGroups.value
    .filter((g) => (g.subscription_type || 'standard') === st)
    .map((g) => g.id)

const isPlatformFullySelected = (platform: GroupPlatform) => {
  const ids = platformGroupIds(platform)
  return ids.length > 0 && ids.every((id) => props.modelValue.includes(id))
}

const isSubscriptionTypeFullySelected = (st: SubscriptionType) => {
  const ids = subscriptionTypeGroupIds(st)
  return ids.length > 0 && ids.every((id) => props.modelValue.includes(id))
}

const toggleIdSet = (ids: number[], currentlyAllSelected: boolean) => {
  if (ids.length === 0) return
  if (currentlyAllSelected) {
    const drop = new Set(ids)
    emit(
      'update:modelValue',
      props.modelValue.filter((id) => !drop.has(id))
    )
    return
  }
  emit('update:modelValue', Array.from(new Set([...props.modelValue, ...ids])))
}

const togglePlatform = (platform: GroupPlatform) => {
  const ids = platformGroupIds(platform)
  toggleIdSet(ids, isPlatformFullySelected(platform))
}

const toggleSubscriptionType = (st: SubscriptionType) => {
  const ids = subscriptionTypeGroupIds(st)
  toggleIdSet(ids, isSubscriptionTypeFullySelected(st))
}

const handleChange = (groupId: number, checked: boolean) => {
  const newValue = checked
    ? [...props.modelValue, groupId]
    : props.modelValue.filter((id) => id !== groupId)
  emit('update:modelValue', newValue)
}

const toggleAllGroups = () => {
  if (filteredGroupIds.value.length === 0) {
    return
  }

  if (allFilteredGroupsSelected.value) {
    const filteredIdSet = new Set(filteredGroupIds.value)
    emit(
      'update:modelValue',
      props.modelValue.filter((groupId) => !filteredIdSet.has(groupId))
    )
    return
  }

  emit('update:modelValue', Array.from(new Set([...props.modelValue, ...filteredGroupIds.value])))
}
</script>
