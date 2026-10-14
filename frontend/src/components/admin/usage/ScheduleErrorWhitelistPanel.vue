<template>
  <div class="space-y-4" data-test="schedule-error-whitelist-card">
    <div class="space-y-2 text-sm leading-6 text-gray-600 dark:text-gray-300">
      <p>{{ t('admin.ops.scheduleErrorWhitelist.description') }}</p>
      <p>{{ t('admin.ops.scheduleErrorWhitelist.checkedHint') }}</p>
      <p>{{ t('admin.errorPolicy.intro') }}</p>
    </div>

    <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500">
      <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <section class="card min-w-0 p-4" data-test="schedule-error-whitelist-custom">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.ops.scheduleErrorWhitelist.customTitle') }}
        </h2>
        <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.scheduleErrorWhitelist.customHint') }}
        </p>

        <div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center">
          <input
            v-model="newMessage"
            type="text"
            maxlength="200"
            class="input min-w-0 flex-1"
            data-test="schedule-error-whitelist-custom-add-input"
            :placeholder="t('admin.ops.scheduleErrorWhitelist.customAddPlaceholder')"
            @keydown.enter.prevent="addMessageRule"
          />
          <button
            type="button"
            class="btn btn-primary shrink-0"
            data-test="schedule-error-whitelist-custom-add"
            :disabled="savingCustom || !loadedOk || !newMessage.trim()"
            @click="addMessageRule"
          >
            {{ t('admin.ops.scheduleErrorWhitelist.customAdd') }}
          </button>
        </div>

        <p
          v-if="form.custom.length === 0"
          class="mt-3 text-sm text-gray-400"
          data-test="schedule-error-whitelist-custom-empty"
        >
          {{ t('admin.ops.scheduleErrorWhitelist.customEmpty') }}
        </p>

        <div v-else class="mt-3 space-y-2">
          <div
            v-for="(rule, index) in form.custom"
            :key="rule.id || index"
            class="rounded-lg border border-gray-100 p-3 dark:border-dark-700"
            :data-test="`schedule-error-whitelist-custom-${index}`"
          >
            <div class="flex items-start gap-2">
              <input
                v-model="rule.enabled"
                type="checkbox"
                class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :disabled="!loadedOk"
                :data-test="`schedule-error-whitelist-custom-enabled-${index}`"
                @change="persistCustom"
              />
              <p class="min-w-0 flex-1 break-words text-sm font-medium text-gray-900 dark:text-white">
                {{ customSummary(rule) }}
              </p>
              <button
                type="button"
                class="shrink-0 text-xs font-semibold text-red-600 hover:text-red-700 disabled:opacity-50"
                :disabled="!loadedOk"
                :data-test="`schedule-error-whitelist-custom-delete-${index}`"
                @click="removeCustom(index)"
              >
                {{ t('common.delete') }}
              </button>
            </div>

            <label
              v-if="isMessageOnly(rule)"
              class="mt-3 block min-w-0 text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.ops.scheduleErrorWhitelist.customMessageLabel') }}
              <input
                v-model="rule.message_contains"
                type="text"
                maxlength="200"
                class="input mt-1 font-mono"
                :disabled="!loadedOk"
                :data-test="`schedule-error-whitelist-custom-message-${index}`"
                @change="persistCustom"
              />
            </label>

            <label class="mt-3 block min-w-0 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.errorPolicy.replaceWith') }}
              <select
                v-model="rule.replace_with"
                :class="selectClass"
                :disabled="!loadedOk"
                :data-test="`error-policy-custom-replace-${index}`"
                @change="persistCustom"
              >
                <option value="">{{ t('admin.errorPolicy.noReplace') }}</option>
                <option v-for="templateId in templateIds" :key="templateId" :value="templateId">
                  {{ templateLabel(templateId) }}
                </option>
              </select>
            </label>
            <p
              v-if="rule.replace_with"
              class="mt-1 break-words text-xs leading-5 text-gray-500 dark:text-gray-400"
            >
              {{ templateSentence(rule.replace_with) }}
            </p>
            <label class="mt-2 flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600"
                :checked="scheduleExcluded(rule)"
                :disabled="!loadedOk"
                :data-test="`error-policy-custom-schedule-${index}`"
                @change="setScheduleExclude(rule, $event)"
              />
              {{ t('admin.errorPolicy.excludeFromSchedule') }}
            </label>
          </div>
        </div>
      </section>

      <div class="grid items-start gap-4 xl:grid-cols-2">
        <section class="card min-w-0 p-4" data-test="error-policy-mappings">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.errorPolicy.mappingsTitle') }}
          </h2>
          <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
            {{ t('admin.errorPolicy.mappingsHint') }}
          </p>
          <div class="mt-3 space-y-2">
            <div
              v-for="rule in form.replacements"
              :key="rule.id"
              class="rounded-lg border border-gray-100 p-3 dark:border-dark-700"
              :data-test="`error-policy-replacement-${rule.id}`"
            >
              <div class="flex items-start gap-2">
                <input
                  v-model="rule.enabled"
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :disabled="rule.locked || !loadedOk"
                  :data-test="`error-policy-replacement-enabled-${rule.id}`"
                />
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ t(`admin.errorPolicy.replacements.${rule.id}`) }}
                  </p>
                  <p class="mt-0.5 font-mono text-xs text-gray-400">
                    {{ replacementMatch(rule) }}
                  </p>
                </div>
              </div>
              <label class="mt-3 block min-w-0 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.errorPolicy.replaceWith') }}
                <select
                  v-model="rule.template"
                  :class="selectClass"
                  :disabled="!loadedOk"
                  :data-test="`error-policy-replacement-template-${rule.id}`"
                >
                  <option v-for="id in templateIds" :key="id" :value="id">
                    {{ templateLabel(id) }}
                  </option>
                </select>
              </label>
              <p v-if="rule.template" class="mt-1 break-words text-xs leading-5 text-gray-500 dark:text-gray-400">
                {{ templateSentence(rule.template) }}
              </p>
              <label v-if="!rule.locked" class="mt-2 flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                <input
                  v-model="rule.schedule_exclude"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600"
                  :disabled="!loadedOk"
                  :data-test="`error-policy-replacement-schedule-${rule.id}`"
                />
                {{ t('admin.errorPolicy.excludeFromSchedule') }}
              </label>
            </div>
          </div>
        </section>

        <section class="card min-w-0 p-4">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.ops.scheduleErrorWhitelist.title') }}
          </h2>
          <div class="mt-3 space-y-2">
            <div
              v-for="id in familyIds"
              :key="id"
              class="rounded-lg border border-gray-100 p-3 dark:border-dark-700"
              :data-test="`schedule-error-whitelist-${id}`"
            >
              <label class="flex items-start gap-2">
                <input
                  v-model="form.families[id]"
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-gray-900 dark:text-white">
                    {{ t(`admin.ops.scheduleErrorWhitelist.families.${id}.label`) }}
                  </span>
                  <span class="mt-0.5 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">
                    {{ t(`admin.ops.scheduleErrorWhitelist.families.${id}.hint`) }}
                  </span>
                </span>
              </label>
              <label class="mt-3 block min-w-0 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.errorPolicy.replaceWith') }}
                <select
                  v-model="form.family_replace[id]"
                  :class="selectClass"
                  :disabled="!loadedOk"
                  :data-test="`error-policy-family-replace-${id}`"
                >
                  <option value="">{{ t('admin.errorPolicy.noReplace') }}</option>
                  <option v-for="templateId in templateIds" :key="templateId" :value="templateId">
                    {{ templateLabel(templateId) }}
                  </option>
                </select>
              </label>
              <p
                v-if="form.family_replace[id]"
                class="mt-1 break-words text-xs leading-5 text-gray-500 dark:text-gray-400"
              >
                {{ templateSentence(form.family_replace[id]) }}
              </p>
            </div>
          </div>
        </section>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          data-test="schedule-error-whitelist-save"
          class="btn btn-primary"
          :disabled="saving"
          @click="save"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CLIENT_ERROR_TEMPLATE_IDS,
  SCHEDULE_ERROR_WHITELIST_FAMILY_IDS,
  defaultScheduleErrorWhitelist,
  getScheduleErrorWhitelist,
  updateScheduleErrorWhitelist,
  type ErrorReplacementRule,
  type ScheduleErrorCustomRule,
  type ScheduleErrorWhitelistFamilyId,
  type ScheduleErrorWhitelistSettings,
  type ScheduleErrorWhitelistUpdate
} from '@/api/admin/settings'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const savingCustom = ref(false)
const loadedOk = ref(false)
const newMessage = ref('')
const familyIds = SCHEDULE_ERROR_WHITELIST_FAMILY_IDS
const templateIds = CLIENT_ERROR_TEMPLATE_IDS
const selectClass =
  'mt-1 block w-full min-w-0 max-w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100'
const form = reactive(defaultScheduleErrorWhitelist())

function cloneCustom(rules: ScheduleErrorCustomRule[] | undefined): ScheduleErrorCustomRule[] {
  return (rules || []).map((rule) => ({ ...rule }))
}

function applySettings(settings: ScheduleErrorWhitelistSettings | null | undefined) {
  const next = defaultScheduleErrorWhitelist()
  for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
    if (settings?.families && typeof settings.families[id] === 'boolean') {
      next.families[id] = settings.families[id]
    }
    form.families[id] = next.families[id]
  }
  form.custom = cloneCustom(settings?.custom).map((rule) => ({
    ...rule,
    replace_with: rule.replace_with || ''
  }))
  form.replacements = cloneReplacements(settings?.replacements)
  for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
    form.family_replace[id] = settings?.family_replace?.[id] || ''
  }
}

function cloneReplacements(rules: ErrorReplacementRule[] | undefined): ErrorReplacementRule[] {
  const source = rules && rules.length > 0 ? rules : defaultScheduleErrorWhitelist().replacements
  return source.map((rule) => ({
    ...rule,
    status_codes: rule.status_codes ? [...rule.status_codes] : undefined
  }))
}

function scheduleExcluded(rule: ScheduleErrorCustomRule): boolean {
  return typeof rule.schedule_exclude === 'boolean' ? rule.schedule_exclude : true
}

function setScheduleExclude(rule: ScheduleErrorCustomRule, event: Event) {
  const target = event.target
  if (!(target instanceof HTMLInputElement)) return
  rule.schedule_exclude = target.checked
  void persistCustom()
}

function templateLabel(id: string): string {
  return t(`admin.errorPolicy.templates.${id}.label`)
}

function templateSentence(id: string): string {
  return t(`admin.errorPolicy.templates.${id}.sentence`)
}

function replacementMatch(rule: ErrorReplacementRule): string {
  if (rule.locked || !rule.status_codes?.length) {
    return t('admin.errorPolicy.catchAll')
  }
  return `HTTP ${rule.status_codes.join(' / ')}`
}

function currentPayload(includePolicy: boolean): ScheduleErrorWhitelistUpdate {
  const payload: ScheduleErrorWhitelistUpdate = { families: currentFamilies() }
  if (!includePolicy) return payload
  payload.custom = cloneCustom(form.custom)
  payload.replacements = cloneReplacements(form.replacements)
  payload.family_replace = { ...form.family_replace }
  return payload
}

function currentFamilies(): Record<ScheduleErrorWhitelistFamilyId, boolean> {
  const families = {} as Record<ScheduleErrorWhitelistFamilyId, boolean>
  for (const id of SCHEDULE_ERROR_WHITELIST_FAMILY_IDS) {
    families[id] = form.families[id] === true
  }
  return families
}

function isMessageOnly(rule: ScheduleErrorCustomRule): boolean {
  return !!String(rule.message_contains || '').trim() &&
    !String(rule.error_type || '').trim() &&
    !String(rule.phase || '').trim() &&
    !rule.status_code &&
    !String(rule.provider_error_code || '').trim()
}

function customSummary(rule: ScheduleErrorCustomRule): string {
  if (isMessageOnly(rule)) {
    return t('admin.ops.scheduleErrorWhitelist.customMessageLabel')
  }
  const parts: string[] = []
  if (rule.error_type) parts.push(`type=${rule.error_type}`)
  if (rule.phase) parts.push(`phase=${rule.phase}`)
  if (rule.status_code) parts.push(`status=${rule.status_code}`)
  if (rule.provider_error_code) parts.push(`code=${rule.provider_error_code}`)
  return parts.join(' · ') || t('admin.ops.scheduleErrorWhitelist.customStructuredLabel')
}

async function load() {
  loading.value = true
  try {
    applySettings(await getScheduleErrorWhitelist())
    loadedOk.value = true
  } catch {
    applySettings(defaultScheduleErrorWhitelist())
    loadedOk.value = false
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    applySettings(await updateScheduleErrorWhitelist(currentPayload(loadedOk.value)))
    appStore.showSuccess(t('admin.ops.scheduleErrorWhitelist.saved'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.ops.scheduleErrorWhitelist.saveFailed'))
    )
  } finally {
    saving.value = false
  }
}

async function persistCustom() {
  if (!loadedOk.value) return
  savingCustom.value = true
  try {
    applySettings(await updateScheduleErrorWhitelist(currentPayload(loadedOk.value)))
    appStore.showSuccess(t('admin.ops.scheduleErrorWhitelist.customSaved'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.ops.scheduleErrorWhitelist.saveFailed'))
    )
    await load()
  } finally {
    savingCustom.value = false
  }
}

async function addMessageRule() {
  const needle = newMessage.value.trim()
  if (!needle) return
  form.custom.push({ enabled: true, message_contains: needle, replace_with: '', schedule_exclude: true })
  newMessage.value = ''
  await persistCustom()
}

async function removeCustom(index: number) {
  form.custom.splice(index, 1)
  await persistCustom()
}

onMounted(() => {
  void load()
})
</script>
