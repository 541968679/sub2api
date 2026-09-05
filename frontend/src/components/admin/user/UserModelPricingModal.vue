<template>
  <BaseDialog :show="show" :title="t('admin.users.modelPricingConfig')" width="wide" @close="$emit('close')">
    <div v-if="user" class="space-y-6">
      <!-- 用户信息头部 -->
      <div class="flex items-center gap-4 rounded-2xl bg-gradient-to-r from-primary-50 to-primary-100 p-5 dark:from-primary-900/30 dark:to-primary-800/20">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white shadow-sm dark:bg-dark-700">
          <span class="text-2xl font-semibold text-primary-600 dark:text-primary-400">{{ user.email.charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flex-1">
          <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ user.email }}</p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{{ t('admin.users.modelPricingHint') }}</p>
          <div class="mt-3 max-w-sm">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">
              {{ t('admin.users.displayCacheTokenMaxMult') }}
            </label>
            <input
              v-model="cacheMaxMultInput"
              type="number"
              min="0"
              max="100"
              step="0.1"
              class="input text-sm"
              :placeholder="t('admin.users.displayCacheTokenMaxMultPlaceholder')"
            />
            <p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('admin.users.displayCacheTokenMaxMultHint') }}
            </p>
          </div>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-12">
        <svg class="h-10 w-10 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <div v-else class="space-y-4">
        <div class="flex flex-wrap items-end gap-x-5 gap-y-0 border-b border-gray-200 dark:border-gray-700">
          <span class="shrink-0 pb-3 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.modelPricing.providerLabel') }}
          </span>
          <button
            v-for="tab in platformTabs"
            :key="'p-' + tab.value"
            type="button"
            class="-mb-px whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.value
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
            :data-test="'platform-tab-' + tab.value"
            @click="activeTab = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <button
            class="flex items-center gap-2 rounded-lg border-2 border-dashed border-gray-300 px-4 py-2 text-sm text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-500 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
            data-test="add-model-override"
            @click="addOverride"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
            {{ t('admin.users.addModelOverride') }}
          </button>
          <button
            v-if="activeTab !== 'other'"
            type="button"
            class="rounded-lg bg-primary-50 px-4 py-2 text-sm font-medium text-primary-700 transition hover:bg-primary-100 dark:bg-primary-900/30 dark:text-primary-300 dark:hover:bg-primary-900/50"
            data-test="bulk-apply-suggested"
            :title="t('admin.users.applySuggestedToCuratedHint')"
            @click="applySuggestedToCurrentPlatform"
          >
            {{ t('admin.users.applySuggestedToCurated') }}
          </button>
        </div>

        <!-- 覆盖列表 -->
        <div
          v-for="item in visibleOverrides"
          :key="item.localKey"
          class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"
          data-test="override-row"
          :data-model="item.model"
        >
          <div class="flex items-center justify-between mb-3">
            <div class="w-64">
              <Select
                v-model="item.model"
                :options="modelOptions"
                :placeholder="t('admin.users.modelNamePlaceholder')"
                searchable
              />
            </div>
            <div class="flex items-center gap-3">
              <label class="flex items-center gap-1.5 text-sm">
                <input v-model="item.enabled" type="checkbox" class="rounded text-primary-500" />
                {{ t('common.enabled') }}
              </label>
              <button class="text-red-500 hover:text-red-700 text-sm" @click="removeOverride(item)">
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <!-- LiteLLM 标准价格参考 -->
          <div v-if="item.model && lookupSuggestedSource(item.model)" class="mb-3 rounded-md bg-blue-50 dark:bg-blue-900/20 p-2 text-xs">
            <div class="font-medium text-blue-700 dark:text-blue-300 mb-1">{{ t('admin.users.litellmReference') }}</div>
            <div class="flex flex-wrap gap-x-4 gap-y-1 text-blue-600 dark:text-blue-400">
              <span>{{ t('admin.modelPricing.inputPrice') }}: {{ getSuggestedMTok(item, 'input_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.outputPrice') }}: {{ getSuggestedMTok(item, 'output_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.cacheWritePrice') }}: {{ getSuggestedMTok(item, 'cache_write_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.cacheWrite1hPrice') }}: {{ getSuggestedMTok(item, 'cache_write_1h_price') ?? '-' }}</span>
              <span>{{ t('admin.modelPricing.cacheReadPrice') }}: {{ getSuggestedMTok(item, 'cache_read_price') ?? '-' }}</span>
              <span class="text-blue-400">($/MTok)</span>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- 真实计费 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.billingPriceOverride') }}</h5>
                <button
                  v-if="item.model && lookupSuggestedSource(item.model)"
                  type="button"
                  class="text-[10px] text-primary-600 hover:text-primary-800 underline dark:text-primary-400 dark:hover:text-primary-300"
                  @click="applySuggestedBilling(item, (field) => getSuggestedMTok(item, field))"
                >
                  {{ t('admin.users.applySuggested') }}
                </button>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.inputPrice') }}</label>
                  <input v-model.number="item.input_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.outputPrice') }}</label>
                  <input v-model.number="item.output_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheWritePrice') }}</label>
                  <input v-model.number="item.cache_write_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500" :title="t('admin.modelPricing.cacheWrite1hPriceHint')">{{ t('admin.modelPricing.cacheWrite1hPrice') }}</label>
                  <input v-model.number="item.cache_write_1h_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheReadPrice') }}</label>
                  <input v-model.number="item.cache_read_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
              </div>
            </div>

            <!-- 展示覆盖 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.displayPriceOverride') }}</h5>
                <button
                  v-if="item.model && lookupSuggestedSource(item.model)"
                  type="button"
                  class="text-[10px] text-primary-600 hover:text-primary-800 underline dark:text-primary-400 dark:hover:text-primary-300"
                  @click="applySuggestedDisplay(item, (field) => getSuggestedMTok(item, field))"
                >
                  {{ t('admin.users.applySuggested') }}
                </button>
              </div>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayInputPrice') }}</label>
                  <input v-model.number="item.display_input_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayOutputPrice') }}</label>
                  <input v-model.number="item.display_output_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500" :title="t('admin.modelPricing.displayCacheCreationPriceHint')">{{ t('admin.users.displayCacheWritePrice') }}</label>
                  <input v-model.number="item.display_cache_creation_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500" :title="t('admin.modelPricing.displayCacheCreation1hPriceHint')">{{ t('admin.users.displayCacheWrite1hPrice') }}</label>
                  <input v-model.number="item.display_cache_creation_1h_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.users.displayCacheReadPrice') }}</label>
                  <input v-model.number="item.display_cache_read_price" type="number" step="any" min="0" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
              </div>
            </div>
          </div>

          <!-- 备注 -->
          <div class="mt-2">
            <input v-model="item.notes" type="text" :placeholder="t('admin.users.notesPlaceholder')"
              class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
          </div>
        </div>

        <div v-if="visibleOverrides.length === 0" class="py-8 text-center text-sm text-gray-400" data-test="empty-overrides">
          {{ t('admin.users.noModelOverridesOnPlatform') }}
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          class="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-700 transition hover:bg-gray-50 dark:border-dark-500 dark:text-gray-300 dark:hover:bg-dark-700"
          @click="$emit('close')"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          class="rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition hover:bg-primary-600 disabled:opacity-50"
          data-test="save-model-pricing"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { mTokToPerToken, perTokenToMTok } from '@/components/admin/channel/types'
import { adminAPI } from '@/api/admin'
import { update as updateUser } from '@/api/admin/users'
import { getPlatformModelCatalog } from '@/api/admin/modelCatalog'
import {
  getUserModelPricing,
  batchUpsertUserModelPricing,
  deleteUserModelPricing,
  type UserModelPricingOverride,
} from '@/api/admin/userModelPricing'
import { MODEL_PRICING_PROVIDER_OPTIONS } from '@/components/admin/model-pricing/modelPricingOptions'
import {
  applySuggestedBilling,
  applySuggestedDisplay,
  applySuggestedToCurated,
  emptyCatalogMap,
  emptyOverrideRow,
  ensureCuratedOverrideRows,
  litellmSuggestedMTok,
  modelSelectOptions,
  rowVisibleOnTab,
  USER_PRICING_PLATFORM_TABS,
  type BillingPriceField,
  type CatalogMap,
  type SuggestedPriceSource,
  type UserModelPricingFormRow,
  type UserPricingTab,
} from '@/components/admin/user/userModelPricingPlatform'

const { t } = useI18n()
const cacheMaxMultInput = ref<string | number>('')

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])

interface ModelInfo extends SuggestedPriceSource {
  model: string
  provider: string
}

const loading = ref(false)
const saving = ref(false)
const overrides = ref<UserModelPricingFormRow[]>([])
const originalIds = ref<Set<number>>(new Set())
const availableModels = ref<ModelInfo[]>([])
const catalogs = ref<CatalogMap>(emptyCatalogMap())
const activeTab = ref<UserPricingTab>('anthropic')

const modelInfoByName = computed(() => {
  const map = new Map<string, ModelInfo>()
  for (const info of availableModels.value) {
    map.set(info.model.trim().toLowerCase(), info)
  }
  return map
})

const platformTabs = computed(() => [
  ...MODEL_PRICING_PROVIDER_OPTIONS,
  { value: 'other' as const, label: t('admin.users.otherPlatform') },
])

const visibleOverrides = computed(() =>
  overrides.value.filter((row) =>
    rowVisibleOnTab(row, activeTab.value, catalogs.value, modelInfoByName.value.get(row.model.trim().toLowerCase()))
  )
)

const modelOptions = computed(() =>
  modelSelectOptions({
    tab: activeTab.value,
    catalogs: catalogs.value,
    available: availableModels.value,
    extraModels: visibleOverrides.value.map((row) => row.model),
  })
)

function lookupSuggestedSource(model: string): ModelInfo | undefined {
  return modelInfoByName.value.get(model.trim().toLowerCase())
}

function getSuggestedMTok(item: UserModelPricingFormRow, field: BillingPriceField): number | null {
  return litellmSuggestedMTok(lookupSuggestedSource(item.model), field)
}

async function loadAvailableModels() {
  if (availableModels.value.length > 0) return
  try {
    const result = await adminAPI.modelPricing.list(1, 1000)
    availableModels.value = (result.items || []).map((i: { model: string; provider?: string; litellm_prices?: SuggestedPriceSource | null }) => ({
      model: i.model,
      provider: i.provider || '',
      input_price: i.litellm_prices?.input_price ?? null,
      output_price: i.litellm_prices?.output_price ?? null,
      cache_write_price: i.litellm_prices?.cache_write_price ?? null,
      cache_write_1h_price: i.litellm_prices?.cache_write_1h_price ?? null,
      cache_read_price: i.litellm_prices?.cache_read_price ?? null,
    }))
  } catch (e) {
    console.error('[UserModelPricing] failed to load model list:', e)
  }
}

async function loadCatalogs() {
  const next = emptyCatalogMap()
  await Promise.all(
    USER_PRICING_PLATFORM_TABS.map(async (platform) => {
      try {
        const catalog = await getPlatformModelCatalog(platform)
        next[platform] = catalog.display_models ?? []
      } catch (e) {
        console.error(`[UserModelPricing] failed to load ${platform} catalog:`, e)
      }
    })
  )
  catalogs.value = next
}

watch(
  () => props.show,
  async (val) => {
    if (!val || !props.user) return
    loading.value = true
    activeTab.value = 'anthropic'
    try {
      cacheMaxMultInput.value =
        props.user.display_cache_token_max_mult != null && props.user.display_cache_token_max_mult > 0
          ? props.user.display_cache_token_max_mult
          : ''
      await Promise.all([loadAvailableModels(), loadCatalogs()])
      const data = await getUserModelPricing(props.user.id)
      overrides.value = (data || []).map((o: UserModelPricingOverride) => ({
        ...emptyOverrideRow(o.model),
        id: o.id,
        localKey: `id-${o.id}`,
        input_price: perTokenToMTok(o.input_price) ?? null,
        output_price: perTokenToMTok(o.output_price) ?? null,
        cache_write_price: perTokenToMTok(o.cache_write_price) ?? null,
        cache_write_1h_price: perTokenToMTok(o.cache_write_1h_price) ?? null,
        cache_read_price: perTokenToMTok(o.cache_read_price) ?? null,
        display_input_price: perTokenToMTok(o.display_input_price) ?? null,
        display_output_price: perTokenToMTok(o.display_output_price) ?? null,
        display_cache_read_price: perTokenToMTok(o.display_cache_read_price) ?? null,
        display_cache_creation_price: perTokenToMTok(o.display_cache_creation_price) ?? null,
        display_cache_creation_1h_price: perTokenToMTok(o.display_cache_creation_1h_price) ?? null,
        enabled: o.enabled,
        notes: o.notes || '',
      }))
      originalIds.value = new Set((data || []).map((o: UserModelPricingOverride) => o.id))
    } catch {
      overrides.value = []
    } finally {
      loading.value = false
    }
  }
)

function addOverride() {
  overrides.value.push(emptyOverrideRow('', activeTab.value))
}

function removeOverride(row: UserModelPricingFormRow) {
  const idx = overrides.value.indexOf(row)
  if (idx >= 0) overrides.value.splice(idx, 1)
}

function applySuggestedToCurrentPlatform() {
  if (activeTab.value === 'other') return
  const curatedIds = catalogs.value[activeTab.value] || []
  overrides.value = ensureCuratedOverrideRows(overrides.value, curatedIds, activeTab.value)
  applySuggestedToCurated(overrides.value, curatedIds, (model, field) =>
    litellmSuggestedMTok(lookupSuggestedSource(model), field)
  )
}

async function save() {
  if (!props.user) return
  saving.value = true
  try {
    const userId = props.user.id
    const currentIds = new Set(overrides.value.filter((o) => o.id).map((o) => o.id!))
    for (const oldId of originalIds.value) {
      if (!currentIds.has(oldId)) {
        await deleteUserModelPricing(userId, oldId)
      }
    }

    const toUpsert = overrides.value
      .filter((o) => o.model.trim())
      .map((o) => ({
        model: o.model.trim(),
        input_price: mTokToPerToken(o.input_price),
        output_price: mTokToPerToken(o.output_price),
        cache_write_price: mTokToPerToken(o.cache_write_price),
        cache_write_1h_price: mTokToPerToken(o.cache_write_1h_price),
        cache_read_price: mTokToPerToken(o.cache_read_price),
        display_input_price: mTokToPerToken(o.display_input_price),
        display_output_price: mTokToPerToken(o.display_output_price),
        display_cache_read_price: mTokToPerToken(o.display_cache_read_price),
        display_cache_creation_price: mTokToPerToken(o.display_cache_creation_price),
        display_cache_creation_1h_price: mTokToPerToken(o.display_cache_creation_1h_price),
        enabled: o.enabled,
        notes: o.notes || '',
      }))

    const modelCounts = new Map<string, number>()
    for (const o of toUpsert) {
      modelCounts.set(o.model, (modelCounts.get(o.model) || 0) + 1)
    }
    const dupes = Array.from(modelCounts.entries()).filter(([, c]) => c > 1).map(([m]) => m)
    if (dupes.length > 0) {
      alert(t('admin.users.duplicateModelError', { models: dupes.join(', ') }))
      saving.value = false
      return
    }
    if (toUpsert.length > 0) {
      await batchUpsertUserModelPricing(userId, toUpsert)
    }

    const rawMult = Number(cacheMaxMultInput.value)
    const multPayload =
      cacheMaxMultInput.value === '' || cacheMaxMultInput.value == null || Number.isNaN(rawMult) || rawMult <= 0
        ? 0
        : rawMult
    await updateUser(userId, { display_cache_token_max_mult: multPayload })

    emit('success')
    emit('close')
  } catch (e) {
    console.error('[UserModelPricing] Save failed:', e)
  } finally {
    saving.value = false
  }
}
</script>
