import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  mergeQualityTemplateFromGate,
  qualityGateFormFromTemplate,
  type QualityGateFormFields
} from '@/utils/accountQualityHardClose'

/** Site-wide quality threshold template. Not bound to a user or account. */
export function useQualityThresholdTemplate() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const templateBusy = ref(false)

  async function applyQualityTemplate(apply: (fields: QualityGateFormFields) => void) {
    if (templateBusy.value) return
    templateBusy.value = true
    try {
      const template = await adminAPI.settings.getQualityHardCloseSettings()
      apply(qualityGateFormFromTemplate(template))
      appStore.showSuccess(t('admin.accounts.stability.applyTemplateSuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.applyTemplateFailed')))
    } finally {
      templateBusy.value = false
    }
  }

  async function saveQualityTemplate(gate: QualityGateFormFields) {
    if (templateBusy.value) return
    templateBusy.value = true
    try {
      const current = await adminAPI.settings.getQualityHardCloseSettings()
      await adminAPI.settings.updateQualityHardCloseSettings(mergeQualityTemplateFromGate(current, gate))
      appStore.showSuccess(t('admin.accounts.stability.saveTemplateSuccess'))
    } catch (error: unknown) {
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.stability.saveTemplateFailed')))
    } finally {
      templateBusy.value = false
    }
  }

  return { templateBusy, applyQualityTemplate, saveQualityTemplate }
}
