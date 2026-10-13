<template>
  <div class="space-y-3">
    <!-- Row 1: Platform tabs (underline style, matching ModelPricingTab) -->
    <div
      v-if="!hidePlatform"
      class="flex flex-wrap items-end gap-x-5 gap-y-0 border-b border-gray-200 dark:border-gray-700"
    >
      <span class="pb-3 text-xs font-medium text-gray-500 shrink-0 dark:text-gray-400">
        {{ t('admin.modelPricing.providerLabel') }}
      </span>
      <button
        v-for="opt in pOpts"
        :key="'p-' + opt.value"
        @click="setPlatform(String(opt.value))"
        class="-mb-px whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
        :class="filters.platform === opt.value
          ? 'border-primary-500 text-primary-600 dark:text-primary-400'
          : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
      >
        {{ opt.label }}
      </button>
    </div>

    <!-- Row 2: Search + other filters -->
    <div class="flex flex-wrap items-center gap-3">
      <SearchInput
        :model-value="searchQuery"
        :placeholder="searchPlaceholder || t('admin.accounts.searchAccounts')"
        class="w-full sm:w-64"
        @update:model-value="$emit('update:searchQuery', $event)"
        @search="$emit('change')"
      />
      <Select :model-value="filters.type" class="w-40" :options="tOpts" @update:model-value="updateType" @change="$emit('change')" />
      <Select :model-value="filters.status" class="w-40" :options="sOpts" @update:model-value="updateStatus" @change="$emit('change')" />
      <Select :model-value="filters.privacy_mode" class="w-40" :options="privacyOpts" @update:model-value="updatePrivacyMode" @change="$emit('change')" />
      <Select
        :model-value="gptGroupValue"
        class="w-40"
        :options="gptGroupOptions"
        data-testid="account-filter-group"
        @update:model-value="updateGroup"
        @change="$emit('change')"
      />
      <Select
        v-if="showDomesticGroupFilter"
        :model-value="domesticGroupValue"
        class="w-44"
        :options="domesticGroupOptions"
        data-testid="account-filter-domestic-group"
        @update:model-value="updateDomesticGroup"
      />
      <button
        v-if="canReorderGroups"
        type="button"
        class="btn btn-secondary shrink-0 px-2.5"
        data-testid="account-group-order-open"
        :title="t('admin.accounts.groupOrder.hint')"
        @click="openGroupOrder"
      >
        <Icon name="menu" size="sm" />
        {{ t('admin.accounts.groupOrder.adjust') }}
      </button>
      <Select v-if="showSort" :model-value="sortValue" class="w-52" :options="sortOpts" @update:model-value="updateSort" />
      <slot name="extra" />
    </div>

  <BaseDialog
    :show="showGroupOrder"
    :title="t('admin.accounts.groupOrder.adjust')"
    width="normal"
    @close="closeGroupOrder"
  >
    <div class="max-h-[60vh] space-y-4 overflow-y-auto" data-testid="account-group-order-dialog">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.groupOrder.hint') }}
      </p>
      <div v-for="(section, sectionIndex) in orderSections" :key="section.key" class="space-y-2">
        <h4 v-if="orderSections.length > 1" class="text-sm font-medium text-gray-700 dark:text-gray-200">
          {{ section.title }}
        </h4>
        <VueDraggable
          :model-value="section.groups"
          :animation="200"
          class="space-y-2"
          @update:model-value="setSectionGroups(sectionIndex, $event)"
        >
          <div
            v-for="(group, index) in section.groups"
            :key="group.id"
            class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-700"
            :data-testid="`account-group-order-row-${group.id}`"
          >
            <Icon name="menu" size="sm" class="shrink-0 cursor-grab text-gray-400" />
            <span class="min-w-0 flex-1 truncate text-sm text-gray-900 dark:text-white">{{ group.name }}</span>
            <button
              type="button"
              class="btn btn-secondary px-2"
              :disabled="index === 0"
              :title="t('admin.accounts.groupOrder.moveUp')"
              :data-testid="`account-group-order-up-${group.id}`"
              @click="moveGroup(sectionIndex, index, -1)"
            >
              <Icon name="chevronUp" size="xs" />
            </button>
            <button
              type="button"
              class="btn btn-secondary px-2"
              :disabled="index === section.groups.length - 1"
              :title="t('admin.accounts.groupOrder.moveDown')"
              :data-testid="`account-group-order-down-${group.id}`"
              @click="moveGroup(sectionIndex, index, 1)"
            >
              <Icon name="chevronDown" size="xs" />
            </button>
          </div>
        </VueDraggable>
      </div>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="closeGroupOrder">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          data-testid="account-group-order-save"
          :disabled="orderSaving"
          @click="saveGroupOrder"
        >
          {{ orderSaving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { VueDraggable } from 'vue-draggable-plus'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup } from '@/types'
import { isDomesticAccountGroup } from '@/utils/domesticAccountGroups'
import { compareAdminGroupsBySort } from '@/utils/groupSortOrder'

const props = withDefaults(defineProps<{
  searchQuery: string
  filters: Record<string, any>
  groups?: AdminGroup[]
  hidePlatform?: boolean
  showSort?: boolean
  searchPlaceholder?: string
  /** Persist a new dropdown order. Absent on screens that only reuse these filters. */
  reorderGroups?: (orderedIds: number[][]) => Promise<void>
}>(), {
  hidePlatform: false,
  showSort: true
})

const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])
const { t } = useI18n()

function setPlatform(value: string) {
  if (props.filters.platform === value) return
  emit('update:filters', { ...props.filters, platform: value })
  emit('change')
}
const updateType = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, type: value }) }
const updateStatus = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, status: value }) }
const updatePrivacyMode = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, privacy_mode: value }) }
const updateGroup = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, group: value == null ? '' : value }) }
const showDomesticGroupFilter = computed(() => props.filters.platform === 'openai')
const sortedGroups = computed(() => [...(props.groups || [])].sort(compareAdminGroupsBySort))
const domesticGroups = computed(() => sortedGroups.value.filter((group) => isDomesticAccountGroup(group)))
const domesticGroupIds = computed(() => new Set(domesticGroups.value.map((group) => String(group.id))))
const selectedGroup = computed(() => String(props.filters.group ?? ''))
const selectedIsDomestic = computed(() => domesticGroupIds.value.has(selectedGroup.value))
const gptGroupValue = computed(() => (selectedIsDomestic.value ? '' : selectedGroup.value))
const domesticGroupValue = computed(() => (selectedIsDomestic.value ? selectedGroup.value : ''))
const updateDomesticGroup = (value: string | number | boolean | null) => {
  const next = value == null ? '' : String(value)
  if (!next) {
    if (!selectedIsDomestic.value) return
    emit('update:filters', { ...props.filters, group: '' })
    emit('change')
    return
  }
  if (next === selectedGroup.value) return
  emit('update:filters', { ...props.filters, group: next })
  emit('change')
}
const sortValue = computed(() => {
  const sortBy = props.filters.sort_by || 'created_at'
  const fallbackOrder = sortBy === 'created_at' || sortBy === 'last_used_at' || sortBy === 'availability' ? 'desc' : 'asc'
  const sortOrder = props.filters.sort_order === 'asc' || props.filters.sort_order === 'desc'
    ? props.filters.sort_order
    : fallbackOrder
  return `${sortBy}:${sortOrder}`
})
const updateSort = (value: string | number | boolean | null) => {
  if (typeof value !== 'string') return
  const [sortBy, sortOrder] = value.split(':')
  emit('update:filters', {
    ...props.filters,
    sort_by: sortBy || 'name',
    sort_order: sortOrder === 'desc' ? 'desc' : 'asc'
  })
  emit('change')
}

const pOpts = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' },
])
const tOpts = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'setup-token', label: t('admin.accounts.setupToken') },
  { value: 'apikey', label: t('admin.accounts.apiKey') },
  { value: 'bedrock', label: 'AWS Bedrock' },
])
const sOpts = computed(() => [
  { value: '', label: t('admin.accounts.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'error', label: t('admin.accounts.status.error') },
  { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') },
  { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') },
  { value: 'unschedulable', label: t('admin.accounts.status.unschedulable') },
])
const privacyOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPrivacyModes') },
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: 'Privacy' },
  { value: 'training_set_cf_blocked', label: 'CF' },
  { value: 'training_set_failed', label: 'Fail' },
])
const groupOption = (group: AdminGroup) => ({ value: String(group.id), label: group.name })
const standardGroups = computed(() => (
  showDomesticGroupFilter.value
    ? sortedGroups.value.filter((group) => !domesticGroupIds.value.has(String(group.id)))
    : sortedGroups.value
))
const gptGroupOptions = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...standardGroups.value.map(groupOption),
])
const canReorderGroups = computed(() => {
  if (!props.reorderGroups) return false
  const lists = showDomesticGroupFilter.value
    ? [standardGroups.value, domesticGroups.value]
    : [sortedGroups.value]
  return lists.some((list) => list.length > 1)
})

interface GroupOrderSection {
  key: string
  title: string
  groups: AdminGroup[]
}

const showGroupOrder = ref(false)
const orderSaving = ref(false)
const orderSections = ref<GroupOrderSection[]>([])

function openGroupOrder() {
  const sections: GroupOrderSection[] = showDomesticGroupFilter.value
    ? [
        { key: 'standard', title: t('admin.accounts.groupOrder.standardSection'), groups: [...standardGroups.value] },
        { key: 'domestic', title: t('admin.accounts.domesticGroups'), groups: [...domesticGroups.value] }
      ]
    : [{ key: 'all', title: t('admin.accounts.groupOrder.standardSection'), groups: [...sortedGroups.value] }]
  orderSections.value = sections.filter((section) => section.groups.length > 0)
  showGroupOrder.value = orderSections.value.length > 0
}

function closeGroupOrder() {
  if (orderSaving.value) return
  showGroupOrder.value = false
}

function setSectionGroups(sectionIndex: number, groups: AdminGroup[]) {
  const section = orderSections.value[sectionIndex]
  if (!section) return
  orderSections.value[sectionIndex] = { ...section, groups }
}

function moveGroup(sectionIndex: number, index: number, direction: -1 | 1) {
  const section = orderSections.value[sectionIndex]
  if (!section) return
  const nextIndex = index + direction
  if (nextIndex < 0 || nextIndex >= section.groups.length) return
  const groups = section.groups.slice()
  const [item] = groups.splice(index, 1)
  if (!item) return
  groups.splice(nextIndex, 0, item)
  setSectionGroups(sectionIndex, groups)
}

async function saveGroupOrder() {
  if (!props.reorderGroups || orderSaving.value) return
  orderSaving.value = true
  try {
    await props.reorderGroups(orderSections.value.map((section) => section.groups.map((group) => group.id)))
    showGroupOrder.value = false
  } finally {
    orderSaving.value = false
  }
}
const domesticGroupOptions = computed(() => [
  { value: '', label: t('admin.accounts.domesticGroups') },
  ...domesticGroups.value.map(groupOption),
])
const sortOpts = computed(() => [
  { value: 'created_at:desc', label: t('admin.accounts.sort.createdDesc') },
  { value: 'created_at:asc', label: t('admin.accounts.sort.createdAsc') },
  { value: 'platform:asc', label: t('admin.accounts.sort.platformAsc') },
  { value: 'type:asc', label: t('admin.accounts.sort.typeAsc') },
  { value: 'availability:desc', label: t('admin.accounts.sort.availabilityDesc') },
  { value: 'availability:asc', label: t('admin.accounts.sort.availabilityAsc') },
  { value: 'name:asc', label: t('admin.accounts.sort.nameAsc') },
  { value: 'last_used_at:desc', label: t('admin.accounts.sort.lastUsedDesc') },
  { value: 'priority:asc', label: t('admin.accounts.sort.priorityAsc') },
  { value: 'upstream_rate_multiplier:asc', label: t('admin.accounts.sort.upstreamRateAsc') },
  { value: 'upstream_rate_multiplier:desc', label: t('admin.accounts.sort.upstreamRateDesc') },
])
</script>
