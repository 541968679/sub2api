<template>
  <div class="space-y-4">
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>
    <template v-else>
      <!-- Action Bar -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <!-- Upload MD file -->
          <label class="btn btn-secondary btn-sm cursor-pointer">
            <svg class="mr-1.5 h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
            </svg>
            {{ t('admin.tutorial.uploadMd') }}
            <input type="file" accept=".md,.markdown,.txt" class="hidden" @change="handleMdUpload" />
          </label>
          <!-- Upload Image -->
          <label class="btn btn-secondary btn-sm cursor-pointer">
            <svg class="mr-1.5 h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 013.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 013.182 0l2.909 2.909M3 3h18a1.5 1.5 0 011.5 1.5v15a1.5 1.5 0 01-1.5 1.5H3a1.5 1.5 0 01-1.5-1.5v-15A1.5 1.5 0 013 3z" />
            </svg>
            {{ t('admin.tutorial.uploadImage') }}
            <input type="file" accept="image/*" class="hidden" @change="handleImageUpload" />
          </label>
          <span v-if="imageUploading" class="text-xs text-gray-500">{{ t('admin.tutorial.uploading') }}</span>
        </div>
        <div class="flex items-center gap-2">
          <button @click="activeView = activeView === 'edit' ? 'preview' : 'edit'" class="btn btn-secondary btn-sm">
            {{ activeView === 'edit' ? t('admin.tutorial.preview') : t('admin.tutorial.edit') }}
          </button>
          <button @click="save" :disabled="saving" class="btn btn-primary btn-sm">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>

      <!-- Local image path warning -->
      <div v-if="localImageWarnings.length" class="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20">
        <p class="text-sm font-medium text-amber-800 dark:text-amber-200">{{ t('admin.tutorial.localImageWarning') }}</p>
        <ul class="mt-1 list-inside list-disc text-xs text-amber-700 dark:text-amber-300">
          <li v-for="w in localImageWarnings" :key="w">{{ w }}</li>
        </ul>
      </div>

      <!-- Editor / Preview -->
      <div v-if="activeView === 'edit'" class="relative">
        <textarea
          v-model="content"
          class="w-full rounded-lg border border-gray-300 bg-white p-4 font-mono text-sm text-gray-900 focus:border-primary-500 focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800 dark:text-white"
          :rows="30"
          :placeholder="t('admin.tutorial.placeholder')"
          @paste="handlePaste"
        ></textarea>
      </div>
      <div v-else class="tutorial-preview max-w-none rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-600 dark:bg-dark-800" v-html="renderedHtml"></div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import { getAdminTutorialContent, updateAdminTutorialContent, uploadTutorialImage } from '@/api/tutorialPage'

const { t } = useI18n()

const content = ref('')
const loading = ref(false)
const saving = ref(false)
const imageUploading = ref(false)
const activeView = ref<'edit' | 'preview'>('edit')

const renderedHtml = computed(() => {
  if (!content.value) return ''
  return marked.parse(content.value, { async: false }) as string
})

const localImageWarnings = computed(() => {
  const warnings: string[] = []
  const localPatterns = /!\[.*?\]\(((?:\.\/|\.\.\/|[A-Za-z]:\\|\/(?!assets\/tutorial\/)).*?)\)/g
  let match
  while ((match = localPatterns.exec(content.value)) !== null) {
    warnings.push(match[1])
  }
  return warnings
})

async function load() {
  loading.value = true
  try {
    const data = await getAdminTutorialContent()
    content.value = data.content
  } catch (e) {
    console.error('Failed to load tutorial content', e)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await updateAdminTutorialContent({ content: content.value })
  } catch (e) {
    console.error('Failed to save tutorial content', e)
  } finally {
    saving.value = false
  }
}

function handleMdUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (e) => {
    content.value = e.target?.result as string
  }
  reader.readAsText(file)
  input.value = ''
}

async function handleImageUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await doUploadImage(file)
  input.value = ''
}

async function handlePaste(event: ClipboardEvent) {
  const items = event.clipboardData?.items
  if (!items) return
  for (const item of items) {
    if (item.type.startsWith('image/')) {
      event.preventDefault()
      const file = item.getAsFile()
      if (file) await doUploadImage(file)
      return
    }
  }
}

async function doUploadImage(file: File) {
  imageUploading.value = true
  try {
    const { url } = await uploadTutorialImage(file)
    const mdImage = `![${file.name}](${url})\n`
    const textarea = document.querySelector('textarea')
    if (textarea) {
      const start = textarea.selectionStart
      content.value = content.value.slice(0, start) + mdImage + content.value.slice(start)
    } else {
      content.value += mdImage
    }
  } catch (e) {
    console.error('Image upload failed', e)
  } finally {
    imageUploading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
@import '@/assets/tutorial-markdown.css';
</style>
