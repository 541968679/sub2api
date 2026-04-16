<template>
  <BaseDialog :show="show" @close="handleClose" :title="t('admin.modelPricing.testModelTitle', { model })" width="normal">
    <div class="space-y-3">
      <!-- Account Select -->
      <div>
        <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
          {{ t('admin.modelPricing.testAccount') }}
        </label>
        <select v-model="testAccountId" class="input w-full text-sm" :disabled="testRunning">
          <option v-if="accounts.length === 0" :value="0">{{ t('admin.modelPricing.testNoAccount') }}</option>
          <option v-for="acc in accounts" :key="acc.id" :value="acc.id">
            {{ acc.credentials?.email || acc.name }} (ID: {{ acc.id }})
          </option>
        </select>
      </div>

      <!-- Prompt -->
      <div>
        <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
          {{ t('admin.modelPricing.testPrompt') }}
        </label>
        <textarea
          v-model="testPrompt"
          rows="2"
          class="input w-full resize-none text-sm"
          :placeholder="t('admin.modelPricing.testPromptPlaceholder')"
          :disabled="testRunning"
        ></textarea>
      </div>

      <!-- Send button -->
      <button
        @click="runTest"
        class="btn btn-primary w-full text-sm"
        :disabled="testRunning || !testAccountId"
      >
        <span v-if="testRunning" class="mr-1.5 inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
        {{ testRunning ? t('admin.modelPricing.testing') : t('admin.modelPricing.sendTest') }}
      </button>

      <!-- Terminal output -->
      <div
        ref="terminalRef"
        class="max-h-[50vh] overflow-y-auto whitespace-pre-wrap rounded-md bg-gray-900 p-3 font-mono text-xs text-green-400"
      >
        <div v-if="testOutput.length === 0" class="text-gray-500">{{ t('admin.modelPricing.testHint') }}</div>
        <div v-for="(line, i) in testOutput" :key="i" :class="line.cls">{{ line.text }}</div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import type { Account } from '@/types'
import { BaseDialog } from '@/components/common'

const props = defineProps<{
  show: boolean
  model: string
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const authStore = useAuthStore()

const accounts = ref<Account[]>([])
const testAccountId = ref<number>(0)
const testPrompt = ref('你好，你是什么模型')
const testRunning = ref(false)
const testOutput = ref<{ text: string; cls: string }[]>([])
const terminalRef = ref<HTMLElement | null>(null)

async function loadAccounts() {
  try {
    const resp = await adminAPI.accounts.list(1, 100, { platform: 'antigravity' })
    const items = resp.items || []
    accounts.value = items.filter((a: Account) =>
      a.status === 'active' &&
      a.schedulable &&
      !a.error_message &&
      !a.temp_unschedulable_until &&
      !a.rate_limited_at
    )
    if (accounts.value.length > 0 && !testAccountId.value) {
      testAccountId.value = accounts.value[0].id
    }
  } catch {
    /* ignore */
  }
}

function appendOutput(text: string, cls = 'text-green-400') {
  testOutput.value.push({ text, cls })
  nextTick(() => {
    if (terminalRef.value) terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  })
}

async function runTest() {
  if (!testAccountId.value || !props.model) return
  testRunning.value = true
  testOutput.value = []

  appendOutput(`> Model: ${props.model}`, 'text-cyan-400')
  const acc = accounts.value.find((a) => a.id === testAccountId.value)
  appendOutput(`> Account: ${acc?.credentials?.email || testAccountId.value}`, 'text-cyan-400')
  appendOutput(`> Prompt: ${testPrompt.value}`, 'text-gray-500')
  appendOutput('', 'text-gray-500')

  try {
    const token = authStore.token
    const resp = await fetch(`/api/v1/admin/accounts/${testAccountId.value}/test`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ model_id: props.model, prompt: testPrompt.value }),
    })

    if (!resp.ok || !resp.body) {
      appendOutput(`Error: HTTP ${resp.status}`, 'text-red-400')
      return
    }

    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const jsonStr = line.slice(6).trim()
        if (!jsonStr || jsonStr === '[DONE]') continue
        try {
          const event = JSON.parse(jsonStr)
          if (event.type === 'content' && event.text) {
            const last = testOutput.value[testOutput.value.length - 1]
            if (last && last.cls === 'text-green-400' && !last.text.startsWith('>')) {
              last.text += event.text
            } else {
              appendOutput(event.text)
            }
            nextTick(() => {
              if (terminalRef.value) terminalRef.value.scrollTop = terminalRef.value.scrollHeight
            })
          } else if (event.type === 'test_start') {
            appendOutput(`> Testing model: ${event.model || props.model}`, 'text-cyan-400')
          } else if (event.type === 'test_complete') {
            appendOutput('')
            if (event.success) {
              appendOutput('> Test passed', 'text-emerald-400')
            } else {
              appendOutput(`> Test failed: ${event.error || 'unknown'}`, 'text-red-400')
            }
          } else if (event.type === 'error') {
            appendOutput(`> Error: ${event.error || event.text || 'unknown'}`, 'text-red-400')
          }
        } catch {
          /* skip malformed JSON */
        }
      }
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    appendOutput(`> Error: ${msg || 'Connection failed'}`, 'text-red-400')
  } finally {
    testRunning.value = false
  }
}

function handleClose() {
  if (testRunning.value) return // 正在测试时不允许关闭
  emit('close')
}

watch(
  () => props.show,
  (val) => {
    if (val) {
      testOutput.value = []
      testAccountId.value = 0
      loadAccounts()
    }
  }
)
</script>
