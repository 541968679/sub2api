<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="card">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accountAutoProvision.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.accountAutoProvision.description') }}
          </p>
        </div>

        <div v-if="loading" class="flex items-center gap-2 px-6 py-8 text-gray-500 dark:text-gray-400">
          <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
          {{ t('common.loading') }}
        </div>

        <div v-else class="space-y-6 p-6">
          <div class="grid gap-4 md:grid-cols-3">
            <label class="space-y-2">
              <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.accountAutoProvision.enabled') }}
              </span>
              <select v-model="form.enabledString" class="input">
                <option value="true">{{ t('common.enabled') }}</option>
                <option value="false">{{ t('common.disabled') }}</option>
              </select>
            </label>

            <label class="space-y-2">
              <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.accountAutoProvision.checkIntervalSeconds') }}
              </span>
              <input v-model.number="form.check_interval_seconds" type="number" min="30" max="3600" class="input" />
            </label>

            <label class="space-y-2">
              <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.accountAutoProvision.maxActionsPerRun') }}
              </span>
              <input v-model.number="form.max_actions_per_run" type="number" min="1" max="20" class="input" />
            </label>
          </div>

          <div class="flex items-center justify-between border-t border-gray-100 pt-6 dark:border-dark-700">
            <div>
              <h2 class="text-lg font-medium text-gray-900 dark:text-white">
                {{ t('admin.accountAutoProvision.rulesTitle') }}
              </h2>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.accountAutoProvision.rulesHint') }}
              </p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addRule">
              {{ t('admin.accountAutoProvision.addRule') }}
            </button>
          </div>

          <div v-if="form.rules.length === 0" class="rounded-lg border border-dashed border-gray-300 px-6 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
            {{ t('admin.accountAutoProvision.emptyRules') }}
          </div>

          <div v-for="(rule, index) in form.rules" :key="rule.id" class="rounded-xl border border-gray-200 dark:border-dark-600">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h3 class="text-base font-medium text-gray-900 dark:text-white">
                  {{ rule.name || t('admin.accountAutoProvision.ruleFallbackName', { index: index + 1 }) }}
                </h3>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ rule.id }}
                </p>
              </div>
              <div class="flex items-center gap-2">
                <select v-model="rule.enabledString" class="input w-28">
                  <option value="true">{{ t('common.enabled') }}</option>
                  <option value="false">{{ t('common.disabled') }}</option>
                </select>
                <button type="button" class="btn btn-secondary btn-sm" @click="removeRule(index)">
                  {{ t('common.delete') }}
                </button>
              </div>
            </div>

            <div class="space-y-5 px-5 py-5">
              <div class="grid gap-4 lg:grid-cols-2">
                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('common.name') }}
                  </span>
                  <input v-model.trim="rule.name" type="text" class="input" />
                </label>

                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.provisionMode') }}
                  </span>
                  <select v-model="rule.provision_mode" class="input">
                    <option value="template">{{ t('admin.accountAutoProvision.provisionModeTemplate') }}</option>
                    <option value="clone_last_healthy">{{ t('admin.accountAutoProvision.provisionModeClone') }}</option>
                  </select>
                </label>
              </div>

              <GroupSelector v-model="rule.group_ids" :groups="groups" />

              <div class="grid gap-4 lg:grid-cols-4">
                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.normalAccountCountBelow') }}
                  </span>
                  <input v-model.number="rule.normal_account_count_below" type="number" min="0" class="input" />
                </label>

                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.concurrencyUtilizationAbove') }}
                  </span>
                  <input v-model.number="rule.concurrency_utilization_above" type="number" min="0" max="100" step="0.1" class="input" />
                </label>

                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.aiCreditsBelow') }}
                  </span>
                  <input v-model.number="rule.ai_credits_below" type="number" min="0" step="0.01" class="input" />
                </label>

                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.cooldownMinutes') }}
                  </span>
                  <input v-model.number="rule.cooldown_minutes" type="number" min="0" max="1440" class="input" />
                </label>
              </div>

              <div class="grid gap-4 lg:grid-cols-3">
                <label class="space-y-2">
                  <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.accountAutoProvision.aiCreditsInterval') }}
                  </span>
                  <select v-model.number="rule.ai_credits_check_interval_minutes" class="input">
                    <option :value="5">5</option>
                    <option :value="15">15</option>
                    <option :value="30">30</option>
                    <option :value="60">60</option>
                  </select>
                </label>
              </div>

              <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-800/60">
                <div class="mb-4 flex items-center justify-between">
                  <div>
                    <h4 class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ t('admin.accountAutoProvision.templateTitle') }}
                    </h4>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.accountAutoProvision.templateHint') }}
                    </p>
                  </div>
                </div>

                <div class="grid gap-4 lg:grid-cols-3">
                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accounts.concurrency') }}
                    </span>
                    <input v-model.number="rule.template.concurrency" type="number" min="1" max="10000" class="input" />
                  </label>

                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accounts.priority') }}
                    </span>
                    <input v-model.number="rule.templatePriorityValue" type="number" min="0" class="input" />
                  </label>

                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accountAutoProvision.loadFactor') }}
                    </span>
                    <input v-model.number="rule.templateLoadFactorValue" type="number" min="0" max="10000" class="input" />
                  </label>

                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accountAutoProvision.proxyId') }}
                    </span>
                    <input v-model.number="rule.templateProxyIdValue" type="number" min="0" class="input" />
                  </label>

                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accountAutoProvision.templateSchedulable') }}
                    </span>
                    <select v-model="rule.templateSchedulableString" class="input">
                      <option value="true">{{ t('common.enabled') }}</option>
                      <option value="false">{{ t('common.disabled') }}</option>
                    </select>
                  </label>

                  <label class="space-y-2">
                    <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('admin.accounts.allowOverages') }}
                    </span>
                    <select v-model="rule.templateAllowOveragesString" class="input">
                      <option value="true">{{ t('common.enabled') }}</option>
                      <option value="false">{{ t('common.disabled') }}</option>
                    </select>
                  </label>
                </div>
              </div>

              <div class="rounded-lg border border-gray-200 px-4 py-3 text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
                {{ t('admin.accountAutoProvision.runtimeHint') }}
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-gray-200 dark:border-dark-600">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-medium text-gray-900 dark:text-white">
                {{ t('admin.accountAutoProvision.runtimeState') }}
              </h2>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.accountAutoProvision.runtimeStateHint') }}
              </p>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('common.name') }}</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('nav.groups') }}</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('admin.accountAutoProvision.lastTriggered') }}</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('admin.accountAutoProvision.lastHealthySnapshot') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="item in runtimeRows" :key="item.key">
                    <td class="px-4 py-3 text-gray-900 dark:text-white">{{ item.ruleName }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ item.groupName }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ item.lastTriggeredText }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ item.snapshotText }}</td>
                  </tr>
                  <tr v-if="runtimeRows.length === 0">
                    <td colspan="4" class="px-4 py-6 text-center text-gray-500 dark:text-gray-400">
                      {{ t('admin.accountAutoProvision.noRuntimeState') }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="rounded-xl border border-gray-200 dark:border-dark-600">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-medium text-gray-900 dark:text-white">
                最近日志
              </h2>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                自动上号最近的撤下、候选跳过和补号执行记录。
              </p>
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800/60">
                  <tr>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">时间</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">动作</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">分组</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">账号</th>
                    <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">详情</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-for="(entry, index) in runtimeState.recent_logs" :key="`${entry.occurred_at}-${entry.action}-${index}`">
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ formatDateTime(entry.occurred_at) }}</td>
                    <td class="px-4 py-3 text-gray-900 dark:text-white">{{ formatLogAction(entry.action) }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ entry.group_name || (entry.group_id ? `#${entry.group_id}` : '-' ) }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ entry.account_name || (entry.account_id ? `#${entry.account_id}` : '-' ) }}</td>
                    <td class="px-4 py-3 text-gray-600 dark:text-gray-300">{{ entry.message }}</td>
                  </tr>
                  <tr v-if="!runtimeState.recent_logs || runtimeState.recent_logs.length === 0">
                    <td colspan="5" class="px-4 py-6 text-center text-gray-500 dark:text-gray-400">
                      暂无日志
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="flex justify-end border-t border-gray-100 pt-6 dark:border-dark-700">
            <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
              <svg
                v-if="saving"
                class="mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
              </svg>
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import adminAPI from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type {
  AccountAutoProvisionSettings,
  AccountAutoProvisionSettingsResponse,
  AccountAutoProvisionRule,
  AccountAutoProvisionState
} from '@/api/admin/settings'
import type { AdminGroup } from '@/types'

type RuleForm = AccountAutoProvisionRule & {
  enabledString: 'true' | 'false'
  templateSchedulableString: 'true' | 'false'
  templateAllowOveragesString: 'true' | 'false'
  templatePriorityValue: number | null
  templateLoadFactorValue: number | null
  templateProxyIdValue: number | null
}

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const groups = ref<AdminGroup[]>([])
const runtimeState = ref<AccountAutoProvisionState>({
  last_triggered: {},
  last_healthy_snapshots: {},
  recent_logs: []
})

const form = reactive({
  enabledString: 'false' as 'true' | 'false',
  check_interval_seconds: 60,
  max_actions_per_run: 1,
  rules: [] as RuleForm[]
})

const notSetLabel = computed(() => {
  const translated = t('common.notSet')
  return translated === 'common.notSet' ? '未设置' : translated
})

const saveSuccessLabel = computed(() => {
  const translated = t('common.saveSuccess')
  return translated === 'common.saveSuccess' ? '保存成功' : translated
})

const load = async () => {
  loading.value = true
  try {
    const [groupList, settings] = await Promise.all([
      adminAPI.groups.getAll(),
      adminAPI.settings.getAccountAutoProvisionSettings()
    ])
    groups.value = groupList
    applyResponse(settings)
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    loading.value = false
  }
}

const applyResponse = (payload: AccountAutoProvisionSettingsResponse) => {
  form.enabledString = payload.config.enabled ? 'true' : 'false'
  form.check_interval_seconds = payload.config.check_interval_seconds
  form.max_actions_per_run = payload.config.max_actions_per_run
  form.rules = payload.config.rules.map(toRuleForm)
  runtimeState.value = payload.state
}

const toRuleForm = (rule: AccountAutoProvisionRule): RuleForm => ({
  ...rule,
  group_ids: [...rule.group_ids],
  template: {
    ...rule.template,
    proxy_id: rule.template.proxy_id ?? null,
    priority: rule.template.priority ?? null,
    load_factor: rule.template.load_factor ?? null
  },
  enabledString: rule.enabled ? 'true' : 'false',
  templateSchedulableString: rule.template.schedulable ? 'true' : 'false',
  templateAllowOveragesString: rule.template.allow_overages ? 'true' : 'false',
  templatePriorityValue: rule.template.priority ?? null,
  templateLoadFactorValue: rule.template.load_factor ?? null,
  templateProxyIdValue: rule.template.proxy_id ?? null
})

const buildRulePayload = (rule: RuleForm): AccountAutoProvisionRule => ({
  id: rule.id,
  name: rule.name.trim(),
  enabled: rule.enabledString === 'true',
  group_ids: [...rule.group_ids],
  normal_account_count_below: Math.max(0, Number(rule.normal_account_count_below || 0)),
  concurrency_utilization_above: Math.max(0, Number(rule.concurrency_utilization_above || 0)),
  ai_credits_below: Math.max(0, Number(rule.ai_credits_below || 0)),
  ai_credits_check_interval_minutes: Number(rule.ai_credits_check_interval_minutes || 15),
  cooldown_minutes: Math.max(0, Number(rule.cooldown_minutes || 0)),
  provision_mode: rule.provision_mode,
  template: {
    concurrency: Math.max(1, Number(rule.template.concurrency || 1)),
    priority: rule.templatePriorityValue ?? null,
    load_factor: rule.templateLoadFactorValue && rule.templateLoadFactorValue > 0 ? rule.templateLoadFactorValue : null,
    proxy_id: rule.templateProxyIdValue && rule.templateProxyIdValue > 0 ? rule.templateProxyIdValue : null,
    schedulable: rule.templateSchedulableString === 'true',
    allow_overages: rule.templateAllowOveragesString === 'true'
  }
})

const save = async () => {
  saving.value = true
  try {
    const payload: AccountAutoProvisionSettings = {
      enabled: form.enabledString === 'true',
      check_interval_seconds: Math.max(30, Number(form.check_interval_seconds || 60)),
      max_actions_per_run: Math.max(1, Number(form.max_actions_per_run || 1)),
      rules: form.rules.map(buildRulePayload)
    }
    const response = await adminAPI.settings.updateAccountAutoProvisionSettings(payload)
    applyResponse(response)
    appStore.showSuccess(saveSuccessLabel.value)
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    saving.value = false
  }
}

const addRule = () => {
  form.rules.push(
    toRuleForm({
      id: `apr-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      name: '',
      enabled: true,
      group_ids: [],
      normal_account_count_below: 1,
      concurrency_utilization_above: 0,
      ai_credits_below: 0,
      ai_credits_check_interval_minutes: 15,
      cooldown_minutes: 15,
      provision_mode: 'template',
      template: {
        concurrency: 10,
        priority: 1,
        load_factor: null,
        proxy_id: null,
        schedulable: true,
        allow_overages: false
      }
    })
  )
}

const removeRule = (index: number) => {
  form.rules.splice(index, 1)
}

const groupNameMap = computed(() => {
  const map = new Map<number, string>()
  for (const group of groups.value) {
    map.set(group.id, group.name)
  }
  return map
})

const runtimeRows = computed(() => {
  const rows: Array<{
    key: string
    ruleName: string
    groupName: string
    lastTriggeredText: string
    snapshotText: string
  }> = []

  for (const rule of form.rules) {
    for (const groupId of rule.group_ids) {
      const key = `${rule.id}:${groupId}`
      const lastTriggered = runtimeState.value.last_triggered?.[key]
      const snapshot = runtimeState.value.last_healthy_snapshots?.[String(groupId)]
      rows.push({
        key,
        ruleName: rule.name || t('admin.accountAutoProvision.ruleFallbackName', { index: rows.length + 1 }),
        groupName: groupNameMap.value.get(groupId) || `#${groupId}`,
        lastTriggeredText: lastTriggered ? new Date(lastTriggered).toLocaleString() : notSetLabel.value,
        snapshotText: snapshot
          ? `${snapshot.source_account_name || `#${snapshot.source_account_id}`} · ${new Date(snapshot.captured_at).toLocaleString()}`
          : notSetLabel.value
      })
    }
  }
  return rows
})

const formatLogAction = (action: string) => {
  switch (action) {
    case 'account_removed':
      return '移出分组'
    case 'candidate_skipped':
      return '候选跳过'
    case 'candidate_applied':
      return '候选补入'
    case 'group_provisioned':
      return '分组补号'
    case 'no_candidate_available':
      return '无可用候选'
    case 'candidate_apply_failed':
      return '补号失败'
    case 'load_group_failed':
      return '读取分组失败'
    case 'load_group_accounts_failed':
      return '读取分组账号失败'
    case 'load_candidates_failed':
      return '读取候选失败'
    default:
      return action
  }
}

onMounted(() => {
  load().catch(() => undefined)
})
</script>
