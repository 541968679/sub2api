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
        <!-- 添加按钮 -->
        <button
          class="flex items-center gap-2 rounded-lg border-2 border-dashed border-gray-300 px-4 py-2 text-sm text-gray-600 transition hover:border-primary-400 hover:text-primary-600 dark:border-dark-500 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
          @click="addOverride"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" /></svg>
          {{ t('admin.users.addModelOverride') }}
        </button>

        <!-- 覆盖列表 -->
        <div v-for="(item, idx) in overrides" :key="idx" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
          <div class="flex items-center justify-between mb-3">
            <input
              v-model="item.model"
              type="text"
              :placeholder="t('admin.users.modelNamePlaceholder')"
              class="w-64 rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-dark-500 dark:bg-dark-700"
            />
            <div class="flex items-center gap-3">
              <label class="flex items-center gap-1.5 text-sm">
                <input v-model="item.enabled" type="checkbox" class="rounded text-primary-500" />
                {{ t('common.enabled') }}
              </label>
              <button class="text-red-500 hover:text-red-700 text-sm" @click="removeOverride(idx)">
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- 真实计费 -->
            <div class="space-y-2">
              <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.billingPriceOverride') }}</h5>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.inputPrice') }}</label>
                  <input v-model.number="item.input_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.outputPrice') }}</label>
                  <input v-model.number="item.output_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheWritePrice') }}</label>
                  <input v-model.number="item.cache_write_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheReadPrice') }}</label>
                  <input v-model.number="item.cache_read_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
              </div>
            </div>

            <!-- 展示覆盖 -->
            <div class="space-y-2">
              <h5 class="text-xs font-semibold text-gray-500 uppercase">{{ t('admin.users.displayPriceOverride') }}</h5>
              <div class="grid grid-cols-2 gap-2">
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayInputPrice') }}</label>
                  <input v-model.number="item.display_input_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayOutputPrice') }}</label>
                  <input v-model.number="item.display_output_price" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.displayRateMultiplier') }}</label>
                  <input v-model.number="item.display_rate_multiplier" type="number" step="any" :placeholder="t('admin.users.noOverride')"
                    class="w-full rounded border border-gray-300 px-2 py-1 text-xs dark:border-dark-500 dark:bg-dark-700" />
                </div>
                <div>
                  <label class="text-xs text-gray-500">{{ t('admin.modelPricing.cacheTransferRatio') }}</label>
                  <input v-model.number="item.cache_transfer_ratio" type="number" step="any" min="0" max="1" :placeholder="t('admin.users.noOverride')"
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

        <div v-if="overrides.length === 0" class="py-8 text-center text-sm text-gray-400">
          {{ t('admin.users.noModelOverrides') }}
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
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  getUserModelPricing,
  batchUpsertUserModelPricing,
  deleteUserModelPricing,
  type UserModelPricingOverride,
} from '@/api/admin/userModelPricing'

const { t } = useI18n()

interface OverrideRow {
  id?: number
  model: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  display_input_price: number | null
  display_output_price: number | null
  display_rate_multiplier: number | null
  cache_transfer_ratio: number | null
  enabled: boolean
  notes: string
  _deleted?: boolean
}

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])

const loading = ref(false)
const saving = ref(false)
const overrides = ref<OverrideRow[]>([])
const originalIds = ref<Set<number>>(new Set())

watch(
  () => props.show,
  async (val) => {
    if (!val || !props.user) return
    loading.value = true
    try {
      const data = await getUserModelPricing(props.user.id)
      overrides.value = (data || []).map((o: UserModelPricingOverride) => ({
        id: o.id,
        model: o.model,
        input_price: o.input_price,
        output_price: o.output_price,
        cache_write_price: o.cache_write_price,
        cache_read_price: o.cache_read_price,
        display_input_price: o.display_input_price,
        display_output_price: o.display_output_price,
        display_rate_multiplier: o.display_rate_multiplier,
        cache_transfer_ratio: o.cache_transfer_ratio,
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
  overrides.value.push({
    model: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    display_input_price: null,
    display_output_price: null,
    display_rate_multiplier: null,
    cache_transfer_ratio: null,
    enabled: true,
    notes: '',
  })
}

function removeOverride(idx: number) {
  overrides.value.splice(idx, 1)
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

    const toUpsert = overrides.value.filter((o) => o.model.trim())
    if (toUpsert.length > 0) {
      await batchUpsertUserModelPricing(userId, toUpsert)
    }

    emit('success')
    emit('close')
  } catch (e) {
    console.error('Save user model pricing failed:', e)
  } finally {
    saving.value = false
  }
}
</script>
