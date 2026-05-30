<template>
  <div>
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
        @click="toggleAllGroups"
      >
        <Icon :name="allFilteredGroupsSelected ? 'x' : 'check'" size="xs" :stroke-width="2" />
        {{ allFilteredGroupsSelected ? t('common.deselectAll') : t('common.selectAll') }}
      </button>
    </div>
    <div
      class="grid max-h-32 grid-cols-2 gap-1 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-800"
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
        class="col-span-2 py-2 text-center text-sm text-gray-500 dark:text-gray-400"
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
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup, GroupPlatform } from '@/types'

const { t } = useI18n()

interface Props {
  modelValue: number[]
  groups: AdminGroup[]
  platform?: GroupPlatform // Optional platform filter
  mixedScheduling?: boolean // For antigravity accounts: allow anthropic/gemini groups
  showToggleAll?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

// Filter groups by platform if specified
const filteredGroups = computed(() => {
  if (!props.platform) {
    return props.groups
  }
  // antigravity 账户启用混合调度后，可选择 anthropic/gemini 分组
  if (props.platform === 'antigravity' && props.mixedScheduling) {
    return props.groups.filter(
      (g) => g.platform === 'antigravity' || g.platform === 'anthropic' || g.platform === 'gemini'
    )
  }
  // 默认：只能选择同 platform 的分组
  return props.groups.filter((g) => g.platform === props.platform)
})

const filteredGroupIds = computed(() => filteredGroups.value.map((group) => group.id))

const allFilteredGroupsSelected = computed(
  () =>
    filteredGroupIds.value.length > 0 &&
    filteredGroupIds.value.every((groupId) => props.modelValue.includes(groupId))
)

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
