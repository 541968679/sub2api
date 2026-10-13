<template>
  <div class="flex min-h-0 flex-1 flex-col" data-test="schedule-error-whitelist-card">
    <p class="text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.scheduleErrorWhitelist.description') }}
    </p>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.scheduleErrorWhitelist.checkedHint') }}
    </p>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.errorPolicy.intro') }}
    </p>

    <div
      v-if="loading"
      class="mt-4 flex items-center gap-2 text-xs text-gray-500"
    >
      <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <div class="mt-3 min-h-0 flex-1 space-y-2 overflow-y-auto">
        <div data-test="error-policy-mappings">
          <p class="text-xs font-medium text-gray-900 dark:text-white">
            {{ t('admin.errorPolicy.mappingsTitle') }}
          </p>
          <p class="mt-1 text-[11px] text-gray-400">
            {{ t('admin.errorPolicy.mappingsHint') }}
          </p>
          <div
            v-for="rule in form.replacements"
            :key="rule.id"
            class="mt-2 rounded-lg border border-gray-100 p-2.5 dark:border-dark-700"
            :data-test="`error-policy-replacement-${rule.id}`"
          >
            <div class="flex items-start gap-2">
              <input
                v-model="rule.enabled"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :disabled="rule.locked || !loadedOk"
                :data-test="`error-policy-replacement-enabled-${rule.id}`"
              />
              <span class="min-w-0 flex-1">
                <span class="block text-xs font-medium text-gray-900 dark:text-white">
                  {{ t(`admin.errorPolicy.replacements.${rule.id}`) }}
                </span>
                <span class="mt-0.5 block font-mono text-[11px] text-gray-400">
                  {{ replacementMatch(rule) }}
                </span>
                <select
                  v-model="rule.template"
                  class="mt-1 w-full rounded border border-gray-200 px-2 py-1 text-[11px] dark:border-dark-600 dark:bg-dark-800"
                  :disabled="!loadedOk"
                  :data-test="`error-policy-replacement-template-${rule.id}`"
                >
                  <option v-for="id in templateIds" :key="id" :value="id">
                    {{ templateOption(id) }}
                  </option>
                </select>
                <label v-if="!rule.locked" class="mt-1 flex items-center gap-2 text-[11px] text-gray-500">
                  <input
                    v-model="rule.schedule_exclude"
                    type="checkbox"
                    class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600"
                    :disabled="!loadedOk"
                    :data-test="`error-policy-replacement-schedule-${rule.id}`"
                  />
                  {{ t('admin.errorPolicy.excludeFromSchedule') }}
                </label>
              </span>
            </div>
          </div>
        </div>

        <label
          v-for="id in familyIds"
          :key="id"
          class="flex items-start gap-2 rounded-lg border border-gray-100 p-2.5 dark:border-dark-700"
          :data-test="`schedule-error-whitelist-${id}`"
        >
          <input
            v-model="form.families[id]"
            type="checkbox"
            class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span>
            <span class="text-xs font-medium text-gray-900 dark:text-white">{{
              t(`admin.ops.scheduleErrorWhitelist.families.${id}.label`)
            }}</span>
            <span class="mt-0.5 block text-[11px] font-normal text-gray-400">
              {{ t(`admin.ops.scheduleErrorWhitelist.families.${id}.hint`) }}
            </span>
            <select
              v-model="form.family_replace[id]"
              class="mt-1 w-full rounded border border-gray-200 px-2 py-1 text-[11px] dark:border-dark-600 dark:bg-dark-800"
              :disabled="!loadedOk"
              :data-test="`error-policy-family-replace-${id}`"
              @click.stop
            >
              <option value="">{{ t('admin.errorPolicy.noReplace') }}</option>
              <option v-for="templateId in templateIds" :key="templateId" :value="templateId">
                {{ templateOption(templateId) }}
              </option>
            </select>
          </span>
        </label>

        <div class="pt-2" data-test="schedule-error-whitelist-custom">
          <p class="text-xs font-medium text-gray-900 dark:text-white">
            {{ t('admin.ops.scheduleErrorWhitelist.customTitle') }}
          </p>
          <p class="mt-1 text-[11px] text-gray-400">
            {{ t('admin.ops.scheduleErrorWhitelist.customHint') }}
          </p>

          <div
            v-if="form.custom.length === 0"
            class="mt-2 text-[11px] text-gray-400"
            data-test="schedule-error-whitelist-custom-empty"
          >
            {{ t('admin.ops.scheduleErrorWhitelist.customEmpty') }}
          </div>

          <div
            v-for="(rule, index) in form.custom"
            :key="rule.id || index"
            class="mt-2 rounded-lg border border-gray-100 p-2.5 dark:border-dark-700"
            :data-test="`schedule-error-whitelist-custom-${index}`"
          >
            <div class="flex items-start gap-2">
              <input
                v-model="rule.enabled"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :disabled="!loadedOk"
                :data-test="`schedule-error-whitelist-custom-enabled-${index}`"
                @change="persistCustom"
              />
              <span class="min-w-0 flex-1">
                <span class="block text-xs font-medium text-gray-900 dark:text-white">
                  {{ customSummary(rule) }}
                </span>
                <input
                  v-if="isMessageOnly(rule)"
                  v-model="rule.message_contains"
                  type="text"
                  maxlength="200"
                  class="mt-1 w-full rounded border border-gray-200 px-2 py-1 font-mono text-[11px] dark:border-dark-600 dark:bg-dark-800"
                  :disabled="!loadedOk"
                  :data-test="`schedule-error-whitelist-custom-message-${index}`"
                  @change="persistCustom"
                />
              </span>
              <span class="mt-1 block">
                <select
                  v-model="rule.replace_with"
                  class="w-full rounded border border-gray-200 px-2 py-1 text-[11px] dark:border-dark-600 dark:bg-dark-800"
                  :disabled="!loadedOk"
                  :data-test="`error-policy-custom-replace-${index}`"
                  @change="persistCustom"
                >
                  <option value="">{{ t('admin.errorPolicy.noReplace') }}</option>
                  <option v-for="templateId in templateIds" :key="templateId" :value="templateId">
                    {{ templateOption(templateId) }}
                  </option>
                </select>
                <label class="mt-1 flex items-center gap-2 text-[11px] text-gray-500">
                  <input
                    type="checkbox"
                    class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600"
                    :checked="scheduleExcluded(rule)"
                    :disabled="!loadedOk"
                    :data-test="`error-policy-custom-schedule-${index}`"
                    @change="setScheduleExclude(rule, $event)"
                  />
                  {{ t('admin.errorPolicy.excludeFromSchedule') }}
                </label>
              </span>
              <button
                type="button"
                class="text-[11px] font-semibold text-red-600 hover:text-red-700 disabled:opacity-50"
                :disabled="!loadedOk"
                :data-test="`schedule-error-whitelist-custom-delete-${index}`"
                @click="removeCustom(index)"
              >
                {{ t('common.delete') }}
              </button>
            </div>
          </div>

          <div class="mt-3 flex items-center gap-2">
            <input
              v-model="newMessage"
              type="text"
              maxlength="200"
              class="min-w-0 flex-1 rounded border border-gray-200 px-2 py-1.5 text-xs dark:border-dark-600 dark:bg-dark-800"
              data-test="schedule-error-whitelist-custom-add-input"
              :placeholder="t('admin.ops.scheduleErrorWhitelist.customAddPlaceholder')"
              @keydown.enter.prevent="addMessageRule"
            />
            <button
              type="button"
              class="rounded-lg bg-gray-900 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50 dark:bg-gray-100 dark:text-gray-900"
              data-test="schedule-error-whitelist-custom-add"
              :disabled="savingCustom || !loadedOk || !newMessage.trim()"
              @click="addMessageRule"
            >
              {{ t('admin.ops.scheduleErrorWhitelist.customAdd') }}
            </button>
          </div>
        </div>
      </div>

      <div class="mt-4 flex justify-end border-t border-gray-100 pt-3 dark:border-dark-700">
        <button
          type="button"
          data-test="schedule-error-whitelist-save"
          class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
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

function templateOption(id: string): string {
  return `${t(`admin.errorPolicy.templates.${id}.label`)} — ${t(`admin.errorPolicy.templates.${id}.sentence`)}`
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
