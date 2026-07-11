<template>
  <AppLayout>
    <TablePageLayout scroll-mode="fixed" :bare-table="activePanel === 'manual'">
      <template #filters>
        <div class="space-y-4">
          <!-- Panel switcher: segmented tabs (page title comes from AppHeader) -->
          <div class="flex flex-wrap items-center gap-3">
            <div
              class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 dark:border-dark-700 dark:bg-dark-800"
              role="tablist"
              :aria-label="t('admin.imageChannelMonitor.title')"
            >
              <button
                type="button"
                role="tab"
                :aria-selected="activePanel === 'monitors'"
                class="rounded-md px-4 py-1.5 text-sm font-medium transition"
                :class="activePanel === 'monitors'
                  ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
                  : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
                @click="activePanel = 'monitors'"
              >
                {{ t('admin.imageChannelMonitor.panels.monitors') }}
              </button>
              <button
                type="button"
                role="tab"
                :aria-selected="activePanel === 'manual'"
                class="rounded-md px-4 py-1.5 text-sm font-medium transition"
                :class="activePanel === 'manual'
                  ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
                  : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
                @click="switchPanel('manual')"
              >
                {{ t('admin.imageChannelMonitor.panels.manual') }}
              </button>
            </div>
          </div>

          <div
            v-if="activePanel === 'monitors'"
            class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between"
          >
            <div class="flex flex-1 flex-col gap-3 sm:flex-row sm:items-center">
              <input
                v-model="searchQuery"
                type="search"
                class="input min-w-0 sm:max-w-xs"
                :placeholder="t('admin.imageChannelMonitor.searchPlaceholder')"
                @input="handleSearch"
              />
              <select v-model="sourceFilter" class="input sm:w-44" @change="reload">
                <option value="">{{ t('admin.imageChannelMonitor.allSources') }}</option>
                <option value="custom">{{ t('admin.imageChannelMonitor.sourceCustom') }}</option>
                <option value="account">{{ t('admin.imageChannelMonitor.sourceAccount') }}</option>
              </select>
              <select v-model="enabledFilter" class="input sm:w-40" @change="reload">
                <option value="">{{ t('admin.imageChannelMonitor.allStatus') }}</option>
                <option value="true">{{ t('admin.imageChannelMonitor.onlyEnabled') }}</option>
                <option value="false">{{ t('admin.imageChannelMonitor.onlyDisabled') }}</option>
              </select>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary" :disabled="loading" @click="reload">
                {{ t('common.refresh') }}
              </button>
              <button type="button" class="btn btn-primary" @click="openCreateDialog">
                {{ t('admin.imageChannelMonitor.createButton') }}
              </button>
            </div>
          </div>
        </div>
      </template>

      <template #table>
        <!-- Manual test console: fixed viewport, internal scroll only -->
        <div
          v-if="activePanel === 'manual'"
          class="flex h-full min-h-0 overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900 max-lg:h-auto max-lg:flex-col max-lg:overflow-visible"
        >
          <!-- LEFT: config -> channels -> persistent CTA -->
          <div class="flex w-[340px] min-h-0 flex-none flex-col border-r border-gray-200 dark:border-dark-700 max-lg:w-full max-lg:border-b max-lg:border-r-0 2xl:w-[380px]">

            <!-- B: parameters (collapsible) -->
            <section class="flex max-h-[52vh] min-h-0 flex-none flex-col border-b border-gray-200 dark:border-dark-700 max-lg:max-h-none">
              <div class="flex flex-none items-center gap-2 px-4 py-2.5">
                <span class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.imageChannelMonitor.manual.config') }}
                </span>
                <div v-if="manualConfigCollapsed" class="flex min-w-0 flex-wrap items-center gap-1">
                  <span
                    v-for="chip in manualConfigSummary"
                    :key="chip"
                    class="truncate rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-dark-800 dark:text-dark-300"
                  >{{ chip }}</span>
                </div>
                <button
                  type="button"
                  class="ml-auto text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300"
                  :aria-expanded="!manualConfigCollapsed"
                  @click="manualConfigCollapsed = !manualConfigCollapsed"
                >
                  {{ manualConfigCollapsed ? t('admin.imageChannelMonitor.manual.expand') : t('admin.imageChannelMonitor.manual.collapse') }}
                </button>
              </div>
              <div v-show="!manualConfigCollapsed" class="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain px-4 pb-4 pr-3">
                <div class="space-y-1.5">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.manual.executionMode') }}</span>
                  <div class="grid grid-cols-3 rounded-lg border border-gray-200 bg-gray-100 p-0.5 dark:border-dark-700 dark:bg-dark-800" role="tablist">
                    <button
                      v-for="option in manualExecutionModeOptions"
                      :key="option.value"
                      type="button"
                      role="tab"
                      :data-testid="`manual-execution-mode-${option.value.replace(/_/g, '-')}`"
                      :aria-selected="manualExecutionMode === option.value"
                      class="min-w-0 rounded-md px-1.5 py-1.5 text-[11px] font-medium transition"
                      :class="manualExecutionMode === option.value
                        ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
                        : 'text-gray-600 hover:text-gray-900 dark:text-dark-300'"
                      @click="setManualExecutionMode(option.value)"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                  <p
                    class="text-[11px] leading-4"
                    :class="manualExecutionMode === 'direct_probe'
                      ? 'text-amber-600 dark:text-amber-300'
                      : 'text-gray-500 dark:text-dark-400'"
                  >
                    {{ manualExecutionModeHint }}
                  </p>
                </div>

                <label v-if="manualExecutionMode !== 'direct_probe'" class="block">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.manual.apiKey') }}</span>
                  <select
                    v-model.number="manualAPIKeyID"
                    data-testid="manual-api-key"
                    class="input"
                    :disabled="manualAPIKeysLoading"
                  >
                    <option :value="0">
                      {{ manualAPIKeysLoading
                        ? t('common.loading')
                        : t('admin.imageChannelMonitor.manual.selectApiKey') }}
                    </option>
                    <option v-for="key in manualAPIKeys" :key="key.id" :value="key.id">
                      {{ key.name || `#${key.id}` }} · {{ t('admin.imageChannelMonitor.manual.apiKeyUser', { id: key.user_id }) }}
                    </option>
                  </select>
                </label>

                <div class="flex items-center justify-between gap-3">
                  <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-0.5 dark:border-dark-700 dark:bg-dark-800" role="tablist">
                    <button
                      v-for="opt in [{ v: 'generate', l: t('admin.imageChannelMonitor.manual.generate') }, { v: 'edit', l: t('admin.imageChannelMonitor.manual.edit') }]"
                      :key="opt.v"
                      type="button"
                      role="tab"
                      :aria-selected="manualForm.mode === opt.v"
                      class="rounded-md px-3 py-1 text-xs font-medium transition"
                      :class="manualForm.mode === opt.v
                        ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
                        : 'text-gray-600 hover:text-gray-900 dark:text-dark-300'"
                      @click="manualForm.mode = opt.v as 'generate' | 'edit'"
                    >
                      {{ opt.l }}
                    </button>
                  </div>
                  <label class="inline-flex items-center gap-1.5 text-xs text-gray-600 dark:text-dark-300">
                    <input
                      v-model="manualForm.download_image"
                      type="checkbox"
                      class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600"
                    />
                    {{ t('admin.imageChannelMonitor.form.downloadImage') }}
                  </label>
                </div>

                <div class="grid grid-cols-2 gap-2.5">
                  <label class="block">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.form.model') }}</span>
                    <input v-model.trim="manualForm.model" class="input" placeholder="gpt-image-1" />
                  </label>
                  <label class="block">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.form.size') }}</span>
                    <select v-model="manualSizeChoice" class="input">
                      <option value="omit">{{ t('admin.imageChannelMonitor.form.sizeModeOmit') }}</option>
                      <option value="auto">{{ t('admin.imageChannelMonitor.form.sizeModeAuto') }}</option>
                      <option v-for="option in standardSizeOptions" :key="option.value" :value="option.value">
                        {{ t(option.labelKey) }}
                      </option>
                      <option value="custom">{{ t('admin.imageChannelMonitor.form.sizeModeCustom') }}</option>
                    </select>
                  </label>
                </div>

                <label v-if="manualForm.size_mode === 'custom'" class="block">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.form.customSize') }}</span>
                  <input
                    v-model.trim="manualForm.custom_size"
                    class="input"
                    :placeholder="t('admin.imageChannelMonitor.form.customSizePlaceholder')"
                  />
                </label>

                <div class="grid grid-cols-2 gap-2.5">
                  <label class="block">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.form.quality') }}</span>
                    <input v-model.trim="manualForm.quality" class="input" placeholder="auto" />
                  </label>
                  <label class="block">
                    <span class="input-label">n</span>
                    <input v-model.number="manualForm.n" type="number" min="1" max="10" class="input" />
                  </label>
                  <label class="block">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.form.timeoutSeconds') }}</span>
                    <input v-model.number="manualForm.timeout_seconds" type="number" min="30" max="600" class="input" />
                  </label>
                  <label class="block">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.manual.concurrency') }}</span>
                    <input
                      v-model.number="manualForm.concurrency"
                      type="number"
                      min="1"
                      :max="manualConcurrencyMax"
                      class="input"
                    />
                  </label>
                </div>

                <label class="block">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.form.responseFormat') }}</span>
                  <select v-model="manualForm.response_format" class="input">
                    <option value="url">URL</option>
                    <option value="b64_json">Base64</option>
                    <option value="">{{ t('admin.imageChannelMonitor.form.responseFormatOmit') }}</option>
                  </select>
                </label>

                <label class="block">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.form.prompt') }}</span>
                  <textarea v-model.trim="manualForm.prompt" rows="2" class="input min-h-[52px]" />
                </label>

                <div v-if="manualForm.mode === 'edit'" class="space-y-2">
                  <div class="flex items-center justify-between gap-2">
                    <span class="input-label">{{ t('admin.imageChannelMonitor.manual.inputImages') }}</span>
                    <span class="text-[11px] tabular-nums text-gray-500 dark:text-dark-400">
                      {{ t('admin.imageChannelMonitor.manual.inputPoolCount', {
                        selected: manualInputImages.length,
                        runs: manualPlannedRunCount,
                      }) }}
                    </span>
                  </div>
                  <input
                    :key="manualImageInputKey"
                    data-testid="manual-input-images"
                    class="input"
                    type="file"
                    accept="image/*"
                    multiple
                    @change="handleManualImageChange"
                  />
                  <div
                    v-if="manualInputImages.length"
                    class="grid max-h-32 grid-cols-2 gap-1.5 overflow-y-auto rounded-md border border-gray-200 p-1.5 text-xs dark:border-dark-700"
                  >
                    <div
                      v-for="(image, index) in manualInputImages"
                      :key="`${image.name}-${index}`"
                      class="flex min-w-0 items-center gap-1 rounded bg-gray-50 p-1 dark:bg-dark-800"
                    >
                      <img :src="image.data" class="h-8 w-8 flex-none rounded object-cover" alt="" />
                      <span class="min-w-0 flex-1 truncate text-gray-600 dark:text-dark-300" :title="image.name">
                        {{ index + 1 }}. {{ image.name }}
                      </span>
                      <button
                        type="button"
                        class="h-6 w-6 flex-none text-gray-400 hover:text-red-600"
                        :title="t('admin.imageChannelMonitor.manual.removeInputImage')"
                        @click="removeManualInputImage(index)"
                      >
                        &times;
                      </button>
                    </div>
                    <button type="button" class="btn btn-secondary btn-sm col-span-2 justify-center" @click="clearManualInputImage">
                      {{ t('common.clear') }}
                    </button>
                  </div>
                  <p v-if="manualInputImages.length < manualRequiredInputImageCount" class="text-[11px] text-amber-600 dark:text-amber-300">
                    {{ t('admin.imageChannelMonitor.manual.inputPoolRequired') }}
                  </p>
                  <p
                    v-else-if="manualInputImages.length < manualPlannedRunCount"
                    class="text-[11px] text-gray-500 dark:text-dark-400"
                  >
                    {{ t('admin.imageChannelMonitor.manual.inputPoolReuseHint') }}
                  </p>
                </div>

                <div class="flex items-center gap-2 border-t border-gray-100 pt-3 dark:border-dark-800">
                  <select v-model="manualPresetSelectedId" class="input flex-1" @change="handleManualPresetSelect">
                    <option value="">{{ t('admin.imageChannelMonitor.manual.selectPreset') }}</option>
                    <option v-for="preset in manualPresets" :key="preset.id" :value="preset.id">
                      {{ preset.name }}
                    </option>
                  </select>
                  <button type="button" class="btn btn-secondary btn-sm" @click="openManualPresetSaveDialog">
                    {{ t('common.save') }}
                  </button>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="!manualPresetSelectedId"
                    @click="deleteManualPreset"
                  >
                    {{ t('admin.imageChannelMonitor.manual.deletePreset') }}
                  </button>
                </div>
              </div>
            </section>

            <!-- C: channels (internal scroll) -->
            <section class="flex min-h-0 flex-1 flex-col">
              <div class="flex flex-wrap items-center gap-2 px-4 pb-2 pt-3">
                <span class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.imageChannelMonitor.manual.targets') }}
                </span>
                <span class="rounded-full border border-primary-200 bg-primary-50 px-2 py-0.5 text-xs font-semibold tabular-nums text-primary-700 dark:border-primary-500/40 dark:bg-primary-900/20 dark:text-primary-200">
                  {{ t('admin.imageChannelMonitor.manual.selectedOfTotal', { selected: manualSelectedTargetCount, total: manualSelectableTargets.length }) }}
                </span>
                <div class="ml-auto flex items-center gap-1">
                  <button type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300" @click="selectAllManualTargets">
                    {{ t('admin.imageChannelMonitor.manual.selectAll') }}
                  </button>
                  <span class="text-gray-300 dark:text-dark-600">·</span>
                  <button type="button" class="text-xs font-medium text-gray-500 hover:text-gray-700 dark:text-dark-400" @click="clearManualTargets">
                    {{ t('admin.imageChannelMonitor.manual.clearSelection') }}
                  </button>
                  <button
                    type="button"
                    class="ml-1 text-xs font-medium text-gray-500 hover:text-gray-700 disabled:opacity-50 dark:text-dark-400"
                    :disabled="manualTargetsLoading"
                    @click="loadManualTargets"
                  >
                    {{ manualTargetsLoading ? t('common.loading') : t('common.refresh') }}
                  </button>
                </div>
              </div>
              <div class="px-4 pb-2">
                <p v-if="manualExecutionMode !== 'direct_probe'" class="mb-2 text-[11px] leading-4 text-gray-500 dark:text-dark-400">
                  {{ manualExecutionMode === 'gateway_group'
                    ? t('admin.imageChannelMonitor.manual.gatewayGroupTargetHint')
                    : t('admin.imageChannelMonitor.manual.gatewayAccountTargetHint') }}
                </p>
                <input
                  v-model.trim="manualTargetSearch"
                  type="search"
                  class="input w-full"
                  :placeholder="t('admin.imageChannelMonitor.manual.searchTargets')"
                />
              </div>
              <div class="min-h-0 flex-1 space-y-1.5 overflow-y-auto px-4 pb-3 max-lg:max-h-[300px]">
                <label
                  v-for="target in manualFilteredTargets"
                  :key="target.id"
                  class="flex cursor-pointer items-center gap-2.5 rounded-lg border p-2 transition"
                  :class="manualSelectedIds.includes(target.id)
                    ? 'border-primary-400 bg-primary-50 dark:border-primary-500/60 dark:bg-primary-900/20'
                    : 'border-gray-200 hover:border-gray-300 dark:border-dark-700 dark:hover:border-dark-600'"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 flex-none rounded border-gray-300 text-primary-600"
                    :value="target.id"
                    :checked="manualSelectedIds.includes(target.id)"
                    @change="handleManualTargetToggle(target.id)"
                  />
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-[13px] font-medium text-gray-900 dark:text-white">{{ target.name }}</span>
                    <span class="flex items-center gap-1.5 truncate text-[11px] text-gray-500 dark:text-dark-400">
                      <span class="rounded px-1 py-0.5 text-[10px] font-medium" :class="sourceBadgeClass(target.source_type)">
                        {{ sourceLabel(target.source_type) }}
                      </span>
                      <span class="truncate">{{ target.model }}</span>
                    </span>
                  </span>
                </label>
                <p v-if="manualFilteredTargets.length === 0" class="py-8 text-center text-xs text-gray-400 dark:text-dark-500">
                  {{ manualTargetsLoading ? t('common.loading') : t('admin.imageChannelMonitor.manual.noTargets') }}
                </p>
              </div>
            </section>

            <!-- Persistent CTA -->
            <section class="flex-none border-t border-gray-200 p-3 dark:border-dark-700">
              <template v-if="!manualRunning">
                <button
                  type="button"
                  class="btn btn-primary w-full justify-center py-2.5 text-sm font-semibold"
                  :disabled="manualStartDisabled"
                  @click="startManualTests"
                >
                  {{ t('admin.imageChannelMonitor.manual.startWithCount', { count: manualPlannedRunCount }) }}
                </button>
                <p class="mt-2 text-center text-[11px] text-gray-400 dark:text-dark-500">
                  {{ manualCTAHint }}
                </p>
              </template>
              <template v-else>
                <div class="mb-2 flex items-center justify-center gap-2 text-sm font-semibold text-blue-600 dark:text-blue-300">
                  <span class="h-4 w-4 animate-spin rounded-full border-2 border-blue-200 border-t-blue-600 dark:border-blue-900 dark:border-t-blue-300"></span>
                  {{ t('admin.imageChannelMonitor.manual.testingProgress', { done: manualBatchStats.done, total: manualBatchStats.total }) }}
                </div>
                <button type="button" class="btn btn-danger w-full justify-center py-2.5 text-sm font-semibold" @click="cancelRunningManualTests">
                  {{ t('admin.imageChannelMonitor.manual.cancelAll') }}
                </button>
              </template>
            </section>
          </div>

          <!-- RIGHT: results (D) -->
          <div class="flex min-h-0 min-w-0 flex-1 flex-col">
            <!-- Running / completion banner -->
            <div
              v-if="manualShowBatchBanner"
              class="flex flex-wrap items-center gap-3 border-b px-4 py-2.5"
              :class="manualRunning
                ? 'border-blue-100 bg-blue-50 dark:border-blue-900/40 dark:bg-blue-900/20'
                : 'border-green-100 bg-green-50 dark:border-green-900/40 dark:bg-green-900/20'"
              role="status"
            >
              <span class="text-sm font-semibold" :class="manualRunning ? 'text-blue-700 dark:text-blue-200' : 'text-green-700 dark:text-green-200'">
                {{ manualRunning
                  ? t('admin.imageChannelMonitor.manual.batchRunning', { total: manualBatchStats.total, done: manualBatchStats.done })
                  : t('admin.imageChannelMonitor.manual.batchComplete', { done: manualBatchStats.done, total: manualBatchStats.total }) }}
              </span>
              <span class="h-1.5 min-w-[100px] flex-1 overflow-hidden rounded-full bg-white/70 dark:bg-dark-800">
                <span
                  class="block h-full rounded-full transition-[width] duration-500"
                  :class="manualRunning ? 'bg-blue-500' : 'bg-green-500'"
                  :style="{ width: `${Math.max(4, manualBatchProgress)}%` }"
                ></span>
              </span>
              <span class="text-xs tabular-nums text-gray-600 dark:text-dark-300">
                {{ t('admin.imageChannelMonitor.manual.resultOk') }} <b class="text-green-600 dark:text-green-300">{{ manualBatchStats.ok }}</b>
                · {{ t('admin.imageChannelMonitor.manual.resultFail') }} <b class="text-red-600 dark:text-red-300">{{ manualBatchStats.failed }}</b>
                <template v-if="manualBatchStats.observationLost">
                  &middot; {{ t('admin.imageChannelMonitor.manual.resultObservationLost') }} <b class="text-amber-600 dark:text-amber-300">{{ manualBatchStats.observationLost }}</b>
                </template>
                <template v-if="manualCurrentBatchAverageMs !== null">
                  · {{ t('admin.imageChannelMonitor.manual.batchAverage') }} <b>{{ formatDuration(manualCurrentBatchAverageMs) }}</b>
                </template>
              </span>
              <button
                v-if="manualRunning"
                type="button"
                class="btn btn-danger btn-sm"
                @click="cancelRunningManualTests"
              >
                {{ t('admin.imageChannelMonitor.manual.cancelAll') }}
              </button>
              <button
                v-else
                type="button"
                class="text-gray-400 hover:text-gray-600 dark:text-dark-500"
                :aria-label="t('common.close')"
                @click="manualBatchDismissed = true"
              >
                ✕
              </button>
            </div>

            <!-- Toolbar -->
            <div class="flex flex-wrap items-center gap-2 border-b border-gray-200 px-4 py-2.5 dark:border-dark-700">
              <div class="mr-auto min-w-0">
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.imageChannelMonitor.manual.records') }}
                </h2>
                <p class="text-[11px] tabular-nums text-gray-500 dark:text-dark-400">
                  {{ t('admin.imageChannelMonitor.manual.recordsSummary', { shown: filteredManualTableEntries.length, total: manualTableEntries.length }) }}
                </p>
              </div>
              <input
                v-model.trim="manualRecordSearch"
                type="search"
                class="input w-40"
                :placeholder="t('admin.imageChannelMonitor.manual.recordSearchPlaceholder')"
              />
              <select v-model="manualRecordStatusFilter" class="input w-auto">
                <option value="">{{ t('admin.imageChannelMonitor.manual.allStatuses') }}</option>
                <option value="running">{{ t('admin.imageChannelMonitor.manual.running') }}</option>
                <option value="operational">{{ t('admin.imageChannelMonitor.status.operational') }}</option>
                <option value="degraded">{{ t('admin.imageChannelMonitor.status.degraded') }}</option>
                <option value="failed">{{ t('admin.imageChannelMonitor.status.failed') }}</option>
                <option value="error">{{ t('admin.imageChannelMonitor.status.error') }}</option>
                <option value="observation_lost">{{ t('admin.imageChannelMonitor.manual.observationLost') }}</option>
                <option value="canceled">{{ t('admin.imageChannelMonitor.manual.canceled') }}</option>
              </select>
              <select v-model="manualRecordModeFilter" class="input w-auto">
                <option value="">{{ t('admin.imageChannelMonitor.manual.allModes') }}</option>
                <option value="generate">{{ t('admin.imageChannelMonitor.manual.generate') }}</option>
                <option value="edit">{{ t('admin.imageChannelMonitor.manual.edit') }}</option>
              </select>
              <select v-model="manualRecordMonitorFilter" class="input w-auto">
                <option value="">{{ t('admin.imageChannelMonitor.manual.allChannels') }}</option>
                <option v-for="option in manualRecordMonitorOptions" :key="option.id" :value="option.id">
                  {{ option.name }}
                </option>
              </select>
              <select v-model="manualRecordSort" class="input w-auto">
                <option value="newest">{{ t('admin.imageChannelMonitor.manual.sortNewest') }}</option>
                <option value="oldest">{{ t('admin.imageChannelMonitor.manual.sortOldest') }}</option>
              </select>
              <details ref="fieldsDetails" class="relative">
                <summary class="btn btn-secondary btn-sm cursor-pointer select-none">
                  {{ t('admin.imageChannelMonitor.manual.visibleFields') }}
                </summary>
                <div class="absolute right-0 z-20 mt-2 w-56 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-700 dark:bg-dark-900">
                  <label
                    v-for="column in manualRecordColumns"
                    :key="column.key"
                    class="flex items-center gap-2 py-1 text-sm text-gray-700 dark:text-dark-200"
                  >
                    <input
                      v-model="manualVisibleColumns"
                      type="checkbox"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600"
                      :value="column.key"
                    />
                    {{ column.label }}
                  </label>
                </div>
              </details>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="manualHistory.length === 0"
                @click="clearManualHistory"
              >
                {{ t('admin.imageChannelMonitor.manual.clearHistory') }}
              </button>
            </div>

            <!-- Results table (internal scroll, sticky header) -->
            <div class="min-h-0 flex-1 overflow-auto">
              <table class="mtbl w-full text-[13px]">
                <thead>
                  <tr>
                    <th v-if="manualColumnVisible('started_at')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.startedAt') }}
                    </th>
                    <th v-if="manualColumnVisible('monitor')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.monitor') }}
                    </th>
                    <th v-if="manualColumnVisible('status')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.status') }}
                    </th>
                    <th v-if="manualColumnVisible('batch')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.batch') }}
                    </th>
                    <th v-if="manualColumnVisible('mode')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.mode') }}
                    </th>
                    <th v-if="manualColumnVisible('model')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.model') }}
                    </th>
                    <th v-if="manualColumnVisible('size')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.size') }}
                    </th>
                    <th v-if="manualColumnVisible('elapsed')" class="mtbl-th mtbl-th-num">
                      {{ t('admin.imageChannelMonitor.manual.columns.elapsed') }}
                    </th>
                    <th v-if="manualColumnVisible('api_total')" class="mtbl-th mtbl-th-num">
                      {{ t('admin.imageChannelMonitor.manual.columns.apiTotal') }}
                    </th>
                    <th v-if="manualColumnVisible('image_download')" class="mtbl-th mtbl-th-num">
                      {{ t('admin.imageChannelMonitor.manual.columns.imageDownload') }}
                    </th>
                    <th v-if="manualColumnVisible('actual_response')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.actualResponse') }}
                    </th>
                    <th v-if="manualColumnVisible('image_info')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.imageInfo') }}
                    </th>
                    <th v-if="manualColumnVisible('exit_ip')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.exitIp') }}
                    </th>
                    <th v-if="manualColumnVisible('output')" class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.output') }}
                    </th>
                    <th class="mtbl-th">
                      {{ t('admin.imageChannelMonitor.manual.columns.actions') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="entry in filteredManualTableEntries"
                    :key="entry.id"
                    class="hover:bg-gray-50 dark:hover:bg-dark-800/60"
                  >
                    <td v-if="manualColumnVisible('started_at')" class="mtbl-td whitespace-nowrap tabular-nums text-gray-600 dark:text-dark-300">
                      {{ formatDate(entry.started_at) }}
                    </td>
                    <td v-if="manualColumnVisible('monitor')" class="mtbl-td">
                      <div class="max-w-[150px] truncate font-medium text-gray-900 dark:text-white" :title="entry.monitor_name">
                        {{ entry.monitor_name }}
                      </div>
                      <div class="text-[11px] tabular-nums text-gray-500 dark:text-dark-400">#{{ entry.monitor_id }}</div>
                    </td>
                    <td v-if="manualColumnVisible('status')" class="mtbl-td">
                      <span class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium" :class="manualRecordBadgeClass(entry)">
                        {{ manualRecordStatusText(entry) }}
                      </span>
                      <div v-if="manualRecordStageText(entry)" class="mt-0.5 max-w-[120px] truncate text-[11px] text-gray-500 dark:text-dark-400" :title="manualRecordStageText(entry)">
                        {{ manualRecordStageText(entry) }}
                      </div>
                      <div v-if="entry.gateway_status" class="mt-0.5 max-w-[160px] truncate text-[10px] text-gray-500 dark:text-dark-400">
                        {{ t('admin.imageChannelMonitor.manual.gatewayStatus') }}: {{ manualStatusValueText(entry.gateway_status) }}
                      </div>
                      <div v-if="entry.delivery_status" class="max-w-[160px] truncate text-[10px] text-gray-500 dark:text-dark-400">
                        {{ t('admin.imageChannelMonitor.manual.deliveryStatus') }}: {{ manualStatusValueText(entry.delivery_status) }}
                      </div>
                      <div
                        v-if="entry.liveItem?.observationWarning || entry.liveItem?.artifactWarning"
                        class="mt-1 max-w-[180px] text-[10px] leading-4 text-amber-600 dark:text-amber-300"
                      >
                        {{ entry.liveItem?.observationWarning || entry.liveItem?.artifactWarning }}
                      </div>
                    </td>
                    <td v-if="manualColumnVisible('batch')" class="mtbl-td whitespace-nowrap">
                      <template v-if="entry.batch_id">
                        <div class="font-mono text-[12px] text-gray-700 dark:text-dark-200" :title="entry.batch_id">
                          {{ shortManualBatchID(entry.batch_id) }} · {{ entry.batch_index }}/{{ entry.batch_size }}
                        </div>
                        <div class="text-[11px] tabular-nums text-gray-500 dark:text-dark-400">
                          {{ t('admin.imageChannelMonitor.manual.batchAverageShort') }} {{ formatDuration(manualRecordBatchAverage(entry)) }}
                        </div>
                      </template>
                      <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                    </td>
                    <td v-if="manualColumnVisible('mode')" class="mtbl-td whitespace-nowrap text-gray-700 dark:text-dark-200">
                      {{ entry.mode === 'edit' ? t('admin.imageChannelMonitor.manual.edit') : t('admin.imageChannelMonitor.manual.generate') }}
                    </td>
                    <td v-if="manualColumnVisible('model')" class="mtbl-td">
                      <div class="max-w-[140px] truncate text-gray-700 dark:text-dark-200" :title="entry.model">
                        {{ entry.model || '-' }}
                      </div>
                    </td>
                    <td v-if="manualColumnVisible('size')" class="mtbl-td whitespace-nowrap tabular-nums text-gray-700 dark:text-dark-200">
                      {{ formatSize(entry.size) }}
                    </td>
                    <td v-if="manualColumnVisible('elapsed')" class="mtbl-td mtbl-td-num text-gray-700 dark:text-dark-200">
                      {{ formatDuration(entry.elapsed_ms) }}
                    </td>
                    <td v-if="manualColumnVisible('api_total')" class="mtbl-td mtbl-td-num text-gray-700 dark:text-dark-200">
                      {{ formatMsGrouped(entry.result?.api_total_ms ?? null) }}
                    </td>
                    <td v-if="manualColumnVisible('image_download')" class="mtbl-td mtbl-td-num text-gray-700 dark:text-dark-200">
                      {{ formatMsGrouped(entry.result?.image_download_ms ?? null) }}
                    </td>
                    <td v-if="manualColumnVisible('actual_response')" class="mtbl-td whitespace-nowrap">
                      <span
                        v-if="manualRecordActualResponseText(entry)"
                        class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium"
                        :class="manualRecordActualResponseBadgeClass(entry)"
                        :title="manualRecordActualResponseTitle(entry)"
                      >
                        {{ manualRecordActualResponseText(entry) }}
                      </span>
                      <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                    </td>
                    <td v-if="manualColumnVisible('image_info')" class="mtbl-td whitespace-nowrap">
                      <template v-if="manualRecordImageDims(entry) || manualRecordImageBytesText(entry)">
                        <div
                          v-if="manualRecordImageDims(entry)"
                          class="tabular-nums"
                          :class="manualRecordSizeMismatch(entry)
                            ? 'font-medium text-amber-600 dark:text-amber-300'
                            : 'text-gray-700 dark:text-dark-200'"
                          :title="manualRecordSizeMismatch(entry)
                            ? t('admin.imageChannelMonitor.manual.sizeMismatch', { size: entry.size })
                            : undefined"
                        >
                          {{ manualRecordImageDims(entry) }}{{ manualRecordSizeMismatch(entry) ? ' ⚠' : '' }}
                        </div>
                        <div v-if="manualRecordImageBytesText(entry)" class="text-[11px] tabular-nums text-gray-500 dark:text-dark-400">
                          {{ manualRecordImageBytesText(entry) }}
                        </div>
                      </template>
                      <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                    </td>
                    <td v-if="manualColumnVisible('exit_ip')" class="mtbl-td">
                      <span class="block max-w-[140px] truncate font-mono text-[12px] text-gray-700 dark:text-dark-200" :title="entry.result?.exit_ip || '-'">
                        {{ entry.result?.exit_ip || '-' }}
                      </span>
                    </td>
                    <td v-if="manualColumnVisible('output')" class="mtbl-td">
                      <div
                        v-if="manualRecordOutputPreview(entry)"
                        class="h-10 w-10 overflow-hidden rounded border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
                      >
                        <img :src="manualRecordOutputPreview(entry)" class="h-full w-full object-cover" alt="" />
                      </div>
                      <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
                    </td>
                    <td class="mtbl-td">
                      <div class="flex items-center gap-1">
                        <button
                          v-if="entry.liveItem?.state === 'running'"
                          type="button"
                          class="rounded px-1.5 py-1 text-[12px] font-medium text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900/30"
                          @click="cancelManualRun(entry.liveItem)"
                        >
                          {{ t('admin.imageChannelMonitor.manual.cancel') }}
                        </button>
                        <button
                          v-if="entry.historyItem?.output_artifact_pending"
                          type="button"
                          class="rounded px-1.5 py-1 text-[12px] font-medium text-amber-600 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-900/20"
                          @click="retryManualArtifact(entry.run_id)"
                        >
                          {{ t('common.retry') }}
                        </button>
                        <button
                          type="button"
                          class="rounded px-1.5 py-1 text-[12px] font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-primary-900/20"
                          @click="openManualRecordDetail(entry)"
                        >
                          {{ t('admin.imageChannelMonitor.manual.viewDetail') }}
                        </button>
                        <a
                          v-if="manualRecordOutputHref(entry)"
                          class="rounded px-1.5 py-1 text-[12px] font-medium text-primary-600 hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-primary-900/20"
                          :href="manualRecordOutputHref(entry)"
                          :download="manualRecordDownloadName(entry)"
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {{ t('admin.imageChannelMonitor.manual.downloadImage') }}
                        </a>
                      </div>
                    </td>
                  </tr>
                  <tr v-if="filteredManualTableEntries.length === 0">
                    <td class="px-3 py-12 text-center text-sm text-gray-500 dark:text-dark-400" :colspan="manualVisibleColumnCount">
                      {{ t('admin.imageChannelMonitor.manual.noRecords') }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <DataTable v-else :columns="columns" :data="monitors" :loading="loading">
          <template #cell-name="{ row, value }">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
                <span
                  v-if="row.api_key_decrypt_failed"
                  class="rounded bg-red-50 px-1.5 py-0.5 text-xs text-red-700 dark:bg-red-900/30 dark:text-red-200"
                >
                  {{ t('admin.imageChannelMonitor.keyError') }}
                </span>
              </div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ row.prompt }}
              </div>
              <div
                class="mt-2 flex flex-wrap items-center gap-2 rounded-md bg-gray-50 px-2 py-1.5 text-xs dark:bg-dark-800"
              >
                <span
                  class="inline-flex rounded px-1.5 py-0.5 font-medium"
                  :class="runtimeBadgeClass(row)"
                >
                  {{ runtimeStateLabel(row) }}
                </span>
                <span class="text-gray-700 dark:text-dark-200">
                  {{ runtimeStageText(row) }}
                </span>
                <span
                  v-if="runtimeMessage(row)"
                  class="max-w-[320px] truncate text-gray-500 dark:text-dark-400"
                  :title="runtimeMessage(row)"
                >
                  {{ runtimeMessage(row) }}
                </span>
                <span class="ml-auto text-gray-500 dark:text-dark-400">
                  {{ nextCheckText(row) }}
                </span>
              </div>
              <div class="mt-2 rounded-md bg-gray-50 px-2 pb-1.5 dark:bg-dark-800">
                <MonitorTimeline
                  :buckets="monitorStripPoints(row)"
                  :countdown-seconds="rowCountdownSeconds(row)"
                />
                <div class="mt-1 flex items-center justify-between text-[11px] text-gray-500 dark:text-dark-400">
                  <span>{{ t('admin.imageChannelMonitor.statusStrip.availability7d') }}</span>
                  <span class="tabular-nums font-medium">{{ formatAvailability(row) }}</span>
                </div>
              </div>
            </div>
          </template>

          <template #cell-source_type="{ row }">
            <div class="space-y-1">
              <span
                class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium"
                :class="sourceBadgeClass(row.source_type)"
              >
                {{ sourceLabel(row.source_type) }}
              </span>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ row.source_type === 'account' ? row.account_name || `#${row.account_id}` : row.endpoint }}
              </div>
              <div
                v-if="row.source_type === 'custom' && row.proxy_id"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ t('admin.imageChannelMonitor.form.proxy') }}:
                {{ row.proxy_name || `#${row.proxy_id}` }}
              </div>
            </div>
          </template>

          <template #cell-model="{ row }">
            <div class="text-sm text-gray-900 dark:text-gray-100">
              {{ row.model }}
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ formatSize(row.size) }} · {{ row.quality }} · n={{ row.n }}
              </div>
            </div>
          </template>

          <template #cell-last_checked_at="{ row }">
            <span class="text-sm text-gray-700 dark:text-dark-200">
              {{ formatDate(row.last_checked_at) }}
            </span>
          </template>

          <template #cell-enabled="{ row }">
            <label class="inline-flex items-center gap-2">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="row.enabled"
                @change="toggleEnabled(row)"
              />
              <span class="text-sm text-gray-700 dark:text-dark-200">
                {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
              </span>
            </label>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap justify-end gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="runningId === row.id"
                @click="runNow(row)"
              >
                {{ runningId === row.id ? t('common.loading') : t('admin.imageChannelMonitor.runNow') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="statusDialogTarget = row">
                {{ t('admin.imageChannelMonitor.statusDetail') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="openHistory(row)">
                {{ t('admin.imageChannelMonitor.history') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="openEditDialog(row)">
                {{ t('common.edit') }}
              </button>
              <button type="button" class="btn btn-danger btn-sm" @click="askDelete(row)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.imageChannelMonitor.noMonitorsYet')"
              :description="t('admin.imageChannelMonitor.createFirstMonitor')"
              :action-text="t('admin.imageChannelMonitor.createButton')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="activePanel === 'monitors' && pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </TablePageLayout>

    <section
      v-if="lastRunResult"
      class="border-t border-gray-200 bg-white px-4 py-5 dark:border-dark-700 dark:bg-dark-900 sm:px-6"
    >
      <div class="mx-auto grid max-w-7xl gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.imageChannelMonitor.lastRun') }}
            </h2>
            <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass(lastRunResult.status)">
              {{ statusLabel(lastRunResult.status) }}
            </span>
          </div>
          <dl class="mt-3 grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
            <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiHeader')" :value="formatMs(lastRunResult.api_header_ms)" />
            <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiBody')" :value="formatMs(lastRunResult.api_body_ms)" />
            <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiTotal')" :value="formatMs(lastRunResult.api_total_ms)" />
            <MetricItem :label="t('admin.imageChannelMonitor.metrics.imageDownload')" :value="formatMs(lastRunResult.image_download_ms)" />
          </dl>
          <p v-if="lastRunResult.message" class="mt-3 text-sm text-red-600 dark:text-red-300">
            {{ lastRunResult.error_stage ? `${lastRunResult.error_stage}: ` : '' }}{{ lastRunResult.message }}
          </p>
        </div>
        <div v-if="lastRunPreview" class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
          <img :src="lastRunPreview" class="aspect-square w-full object-contain" alt="" />
        </div>
      </div>
    </section>

    <BaseDialog
      :show="showDialog"
      :title="editing ? t('admin.imageChannelMonitor.editTitle') : t('admin.imageChannelMonitor.createTitle')"
      width="wide"
      @close="closeDialog"
    >
      <form class="space-y-5" @submit.prevent="saveMonitor">
        <div class="grid gap-4 md:grid-cols-2">
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.name') }}</span>
            <input v-model.trim="form.name" class="input" required />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.source') }}</span>
            <select v-model="form.source_type" class="input" @change="handleSourceChange">
              <option value="custom">{{ t('admin.imageChannelMonitor.sourceCustom') }}</option>
              <option value="account">{{ t('admin.imageChannelMonitor.sourceAccount') }}</option>
            </select>
          </label>
        </div>

        <div v-if="form.source_type === 'custom'" class="grid gap-4 md:grid-cols-3">
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.endpoint') }}</span>
            <input v-model.trim="form.endpoint" class="input" placeholder="https://api.openai.com" required />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.apiKey') }}</span>
            <input
              v-model="form.api_key"
              type="password"
              class="input"
              :placeholder="editing ? t('admin.imageChannelMonitor.form.apiKeyEditPlaceholder') : ''"
              :required="!editing"
            />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.proxy') }}</span>
            <select v-model.number="form.proxy_id" class="input" :disabled="proxiesLoading">
              <option :value="0">{{ t('admin.imageChannelMonitor.form.noProxy') }}</option>
              <option v-for="proxy in proxyOptions" :key="proxy.id" :value="proxy.id">
                {{ proxy.name }} ({{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }})
              </option>
            </select>
          </label>
        </div>

        <div v-else class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.account') }}</span>
            <select v-model.number="form.account_id" class="input" required>
              <option :value="null">{{ t('admin.imageChannelMonitor.form.selectAccount') }}</option>
              <option v-for="account in accountOptions" :key="account.id" :value="account.id">
                {{ account.name }} (#{{ account.id }})
              </option>
            </select>
          </label>
          <button type="button" class="btn btn-secondary" :disabled="accountsLoading" @click="loadAccountOptions">
            {{ accountsLoading ? t('common.loading') : t('common.refresh') }}
          </button>
        </div>

        <div class="grid gap-4 md:grid-cols-4">
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.model') }}</span>
            <input v-model.trim="form.model" class="input" placeholder="gpt-image-1" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.sizeMode') }}</span>
            <select v-model="form.size_mode" class="input">
              <option value="omit">{{ t('admin.imageChannelMonitor.form.sizeModeOmit') }}</option>
              <option value="auto">{{ t('admin.imageChannelMonitor.form.sizeModeAuto') }}</option>
              <option value="preset">{{ t('admin.imageChannelMonitor.form.sizeModePreset') }}</option>
              <option value="custom">{{ t('admin.imageChannelMonitor.form.sizeModeCustom') }}</option>
            </select>
          </label>
          <label v-if="form.size_mode === 'preset'" class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.size') }}</span>
            <select v-model="form.size" class="input">
              <option v-for="option in standardSizeOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </label>
          <label v-else-if="form.size_mode === 'custom'" class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.customSize') }}</span>
            <input
              v-model.trim="form.custom_size"
              class="input"
              :placeholder="t('admin.imageChannelMonitor.form.customSizePlaceholder')"
            />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.quality') }}</span>
            <input v-model.trim="form.quality" class="input" placeholder="auto" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.responseFormat') }}</span>
            <select v-model="form.response_format" class="input">
              <option value="url">URL</option>
              <option value="b64_json">Base64</option>
              <option value="">{{ t('admin.imageChannelMonitor.form.responseFormatOmit') }}</option>
            </select>
          </label>
        </div>

        <label class="block">
          <span class="input-label">{{ t('admin.imageChannelMonitor.form.prompt') }}</span>
          <textarea v-model.trim="form.prompt" class="input min-h-[96px]" />
        </label>

        <div class="grid gap-4 md:grid-cols-4">
          <label class="block">
            <span class="input-label">n</span>
            <input v-model.number="form.n" type="number" min="1" max="10" class="input" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.intervalSeconds') }}</span>
            <input v-model.number="form.interval_seconds" type="number" min="15" max="3600" class="input" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.imageChannelMonitor.form.timeoutSeconds') }}</span>
            <input v-model.number="form.timeout_seconds" type="number" min="30" max="600" class="input" />
          </label>
          <div class="flex flex-col justify-end gap-2">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="form.download_image" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.imageChannelMonitor.form.downloadImage') }}
            </label>
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.imageChannelMonitor.form.enabled') }}
            </label>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.imageChannelMonitor.form.publicSection') }}
          </div>
          <div class="grid gap-4 md:grid-cols-2 md:items-end">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="form.public_visible" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.imageChannelMonitor.form.publicVisible') }}
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.imageChannelMonitor.form.publicName') }}</span>
              <input
                v-model.trim="form.public_name"
                class="input"
                maxlength="200"
                :placeholder="t('admin.imageChannelMonitor.form.publicNamePlaceholder')"
              />
            </label>
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.imageChannelMonitor.form.publicHint') }}
          </p>
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeDialog">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="saveMonitor">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showHistoryDialog"
      :title="t('admin.imageChannelMonitor.history')"
      width="extra-wide"
      @close="showHistoryDialog = false"
    >
      <div class="overflow-x-auto">
        <table class="w-full min-w-[840px] divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead>
            <tr class="text-left text-xs uppercase text-gray-500 dark:text-dark-400">
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.columns.status') }}</th>
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.form.responseFormat') }}</th>
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.metrics.apiTotal') }}</th>
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.metrics.imageDownload') }}</th>
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.columns.image') }}</th>
              <th class="py-2 pr-4">{{ t('admin.imageChannelMonitor.columns.message') }}</th>
              <th class="py-2">{{ t('admin.imageChannelMonitor.columns.checkedAt') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
            <tr v-for="item in historyItems" :key="item.id">
              <td class="py-3 pr-4">
                <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass(item.status)">
                  {{ statusLabel(item.status) }}
                </span>
              </td>
              <td class="py-3 pr-4 whitespace-nowrap">{{ responseFormatLabel(item.response_format) }}</td>
              <td class="py-3 pr-4">{{ formatMs(item.api_total_ms) }}</td>
              <td class="py-3 pr-4">{{ formatMs(item.image_download_ms) }}</td>
              <td class="whitespace-nowrap py-3 pr-4">
                {{ historyImageInfo(item) }}
              </td>
              <td class="max-w-md py-3 pr-4 text-gray-600 dark:text-dark-300">
                {{ item.message || item.error_stage || '-' }}
              </td>
              <td class="py-3">{{ formatDate(item.checked_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseDialog>

    <BaseDialog
      :show="Boolean(selectedManualRecord)"
      :title="t('admin.imageChannelMonitor.manual.recordDetail')"
      width="extra-wide"
      @close="selectedManualRecord = null"
    >
      <div v-if="selectedManualRecord" class="max-h-[70vh] overflow-y-auto pr-1">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                {{ selectedManualRecord.monitor_name }}
              </h3>
              <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="manualRecordBadgeClass(selectedManualRecord)">
                {{ manualRecordStatusText(selectedManualRecord) }}
              </span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ formatDate(selectedManualRecord.started_at) }} - {{ formatDate(selectedManualRecord.completed_at) }}
            </p>
          </div>
          <div class="text-right text-xs text-gray-500 dark:text-dark-400">
            <div>{{ t('admin.imageChannelMonitor.manual.elapsed') }} {{ formatDuration(selectedManualRecord.elapsed_ms) }}</div>
            <div v-if="manualRecordStageText(selectedManualRecord)">{{ manualRecordStageText(selectedManualRecord) }}</div>
          </div>
        </div>

        <!-- Latency waterfall: header -> body -> image download -->
        <div v-if="selectedRecordWaterfall.length" class="mt-4">
          <div class="mb-1.5 text-xs font-medium text-gray-500 dark:text-dark-400">
            {{ t('admin.imageChannelMonitor.manual.waterfall') }}
          </div>
          <div class="flex h-6 overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
            <span
              v-for="seg in selectedRecordWaterfall"
              :key="seg.key"
              class="h-full"
              :class="waterfallSegmentClass[seg.key]"
              :style="{ width: `${seg.pct}%` }"
              :title="`${seg.label} ${seg.ms.toLocaleString()} ms`"
            ></span>
          </div>
          <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-600 dark:text-dark-300">
            <span v-for="seg in selectedRecordWaterfall" :key="`legend-${seg.key}`" class="inline-flex items-center gap-1.5">
              <span class="h-2.5 w-2.5 rounded-sm" :class="waterfallSegmentClass[seg.key]"></span>
              {{ seg.label }} <b class="tabular-nums">{{ seg.ms.toLocaleString() }} ms</b>
            </span>
            <span class="ml-auto font-medium text-gray-700 dark:text-dark-200">
              {{ t('admin.imageChannelMonitor.metrics.apiTotal') }}
              <b class="tabular-nums">{{ formatMsGrouped(selectedManualRecord.result?.api_total_ms ?? null) }}</b>
            </span>
          </div>
        </div>

        <dl class="mt-4 grid gap-3 text-sm sm:grid-cols-3 md:grid-cols-6">
          <MetricItem :label="t('admin.imageChannelMonitor.manual.mode')" :value="selectedManualRecord.mode === 'edit' ? t('admin.imageChannelMonitor.manual.edit') : t('admin.imageChannelMonitor.manual.generate')" />
          <MetricItem
            v-if="selectedManualRecord.batch_id"
            :label="t('admin.imageChannelMonitor.manual.batch')"
            :value="shortManualBatchID(selectedManualRecord.batch_id)"
          />
          <MetricItem
            v-if="selectedManualRecord.batch_id"
            :label="t('admin.imageChannelMonitor.manual.batchIndex')"
            :value="`${selectedManualRecord.batch_index}/${selectedManualRecord.batch_size}`"
          />
          <MetricItem
            v-if="selectedManualRecord.batch_id"
            :label="t('admin.imageChannelMonitor.manual.batchAverage')"
            :value="formatDuration(manualRecordBatchAverage(selectedManualRecord))"
          />
          <MetricItem :label="t('admin.imageChannelMonitor.form.model')" :value="selectedManualRecord.model || '-'" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.size')" :value="formatSize(selectedManualRecord.size)" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.quality')" :value="selectedManualRecord.quality || '-'" />
          <MetricItem :label="'n'" :value="String(selectedManualRecord.n)" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.downloadImage')" :value="selectedManualRecord.download_image ? t('common.yes') : t('common.no')" />
          <MetricItem
            :label="t('admin.imageChannelMonitor.form.responseFormat')"
            :value="responseFormatLabel(selectedManualRecord.response_format)"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.executionMode')"
            :value="manualExecutionModeLabel(selectedManualRecord.execution_mode)"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.gatewayStatus')"
            :value="manualStatusValueText(selectedManualRecord.gateway_status) || '-'"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.deliveryStatus')"
            :value="manualStatusValueText(selectedManualRecord.delivery_status) || '-'"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.actualResponse')"
            :value="manualRecordActualResponseText(selectedManualRecord) || '-'"
            :tone="manualRecordActualResponseKind(selectedManualRecord).includes('DataUrl') ? 'warn' : undefined"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.actualSize')"
            :value="manualRecordImageDims(selectedManualRecord) || '-'"
            :tone="manualRecordSizeMismatch(selectedManualRecord) ? 'warn' : undefined"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.imageBytes')"
            :value="manualRecordImageBytesText(selectedManualRecord) || '-'"
          />
          <MetricItem
            :label="t('admin.imageChannelMonitor.manual.imageFormat')"
            :value="selectedManualRecord.result?.image_content_type || '-'"
          />
        </dl>
        <p
          v-if="manualRecordSizeMismatch(selectedManualRecord)"
          class="mt-2 text-xs font-medium text-amber-600 dark:text-amber-300"
        >
          {{ t('admin.imageChannelMonitor.manual.sizeMismatch', { size: selectedManualRecord.size }) }}
        </p>

        <div class="mt-4 rounded-md bg-gray-50 p-3 text-sm text-gray-700 dark:bg-dark-800 dark:text-dark-200">
          <div class="text-xs font-medium text-gray-500 dark:text-dark-400">
            {{ t('admin.imageChannelMonitor.form.prompt') }}
          </div>
          <p class="mt-1 whitespace-pre-wrap break-words">{{ selectedManualRecord.prompt || '-' }}</p>
        </div>

        <div
          v-if="networkInfoItems(selectedManualRecord.result).length"
          class="mt-4 grid gap-2 rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300 md:grid-cols-2"
        >
          <div
            v-for="info in networkInfoItems(selectedManualRecord.result)"
            :key="`${selectedManualRecord.id}-${info.label}`"
            class="min-w-0"
          >
            <div class="font-medium text-gray-500 dark:text-dark-400">{{ info.label }}</div>
            <a
              v-if="info.href"
              class="block truncate text-primary-600 hover:text-primary-700 dark:text-primary-300"
              :href="info.href"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ info.value }}
            </a>
            <div v-else class="truncate" :title="info.value">{{ info.value }}</div>
          </div>
        </div>

        <p v-if="selectedManualRecord.message" class="mt-3 text-sm text-red-600 dark:text-red-300">
          {{ selectedManualRecord.stage ? `${selectedManualRecord.stage}: ` : '' }}{{ selectedManualRecord.message }}
        </p>

        <div class="mt-4 grid gap-4 md:grid-cols-2">
          <div>
            <div class="mb-2 text-xs font-medium text-gray-500 dark:text-dark-400">
              {{ t('admin.imageChannelMonitor.manual.inputImage') }}
            </div>
            <div
              v-if="manualRecordInputPreview(selectedManualRecord)"
              class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
            >
              <img :src="manualRecordInputPreview(selectedManualRecord)" class="max-h-80 w-full object-contain" alt="" />
            </div>
            <div v-else class="rounded-lg border border-dashed border-gray-200 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.imageChannelMonitor.manual.noImage') }}
            </div>
          </div>
          <div>
            <div class="mb-2 text-xs font-medium text-gray-500 dark:text-dark-400">
              {{ t('admin.imageChannelMonitor.manual.outputImage') }}
            </div>
            <div
              v-if="manualRecordOutputPreview(selectedManualRecord)"
              class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
            >
              <img :src="manualRecordOutputPreview(selectedManualRecord)" class="max-h-80 w-full object-contain" alt="" />
            </div>
            <a
              v-else-if="selectedManualRecord.outputUrl"
              class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-300"
              :href="selectedManualRecord.outputUrl"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ selectedManualRecord.outputUrl }}
            </a>
            <div v-else class="rounded-lg border border-dashed border-gray-200 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
              {{ t('admin.imageChannelMonitor.manual.noImage') }}
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <a
          v-if="manualRecordOutputHref(selectedManualRecord)"
          class="btn btn-secondary"
          :href="manualRecordOutputHref(selectedManualRecord)"
          :download="manualRecordDownloadName(selectedManualRecord)"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ t('admin.imageChannelMonitor.manual.downloadImage') }}
        </a>
        <button type="button" class="btn btn-primary" @click="selectedManualRecord = null">
          {{ t('common.close') }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showManualPresetSaveDialog"
      :title="t('admin.imageChannelMonitor.manual.savePresetTitle')"
      @close="showManualPresetSaveDialog = false"
    >
      <label class="block">
        <span class="input-label">{{ t('admin.imageChannelMonitor.manual.presetName') }}</span>
        <input
          v-model.trim="manualPresetName"
          class="input"
          :placeholder="t('admin.imageChannelMonitor.manual.presetNamePlaceholder')"
          @keyup.enter="saveManualPreset"
        />
      </label>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showManualPresetSaveDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" @click="saveManualPreset">
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <ImageMonitorStatusDialog
      :show="Boolean(statusDialogTarget)"
      :monitor="statusDialogTarget"
      @close="statusDialogTarget = null"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes } from '@/utils/format'
import {
  buildManualRunRequests,
  isManualRetryableRequestError,
  manualArtifactRecoveryExpiresAt,
  manualRunObservationMaxAttempts,
  ManualRunObservationError,
  pollManualRunUntilTerminal,
  revokeManualObjectURLsForRun,
} from '@/utils/imageChannelManualTest'
import type { Account, Proxy } from '@/types'
import type { SimpleApiKey } from '@/api/admin/usage'
import type { Column } from '@/components/common/types'
import type {
  ImageChannelMonitor,
  ImageChannelMonitorHistoryItem,
  ImageChannelMonitorListParams,
  ImageChannelManualRunResponse,
  ImageChannelManualTestParams,
  ImageChannelMonitorResult,
  ImageChannelMonitorRuntimeStatus,
  ImageMonitorResponseFormat,
  ImageMonitorSourceType,
  ImageMonitorStatus,
} from '@/api/admin/imageChannelMonitor'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import ImageMonitorStatusDialog from '@/components/admin/ImageMonitorStatusDialog.vue'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const MetricItem = (_props: { label: string; value: string; tone?: 'warn' }) =>
  h('div', { class: 'rounded-md bg-gray-50 p-3 dark:bg-dark-800' }, [
    h('dt', { class: 'text-xs text-gray-500 dark:text-dark-400' }, _props.label),
    h(
      'dd',
      {
        class: [
          'mt-1 font-medium',
          _props.tone === 'warn'
            ? 'text-amber-600 dark:text-amber-300'
            : 'text-gray-900 dark:text-white',
        ],
      },
      _props.value
    ),
  ])

const { t } = useI18n()
const appStore = useAppStore()

type ImageSizeMode = 'omit' | 'auto' | 'preset' | 'custom'
type ImageMonitorPanel = 'monitors' | 'manual'
type ManualExecutionMode = 'gateway_account' | 'gateway_group' | 'direct_probe'

type ManualResultItem = {
  recordId: string
  monitor: ImageChannelMonitor
  state: 'running' | 'done' | 'error' | 'canceled' | 'observation_lost'
  message: string
  observationWarning?: string
  artifactWarning?: string
  batch_id: string
  batch_size: number
  batch_index: number
  clientRunID?: string
  run?: ImageChannelManualRunResponse
  settings?: ManualPresetSettings
  inputImage?: ManualInputImage | null
  startedAt?: string
  completedAt?: string
}

type ManualPresetSettings = {
  execution_mode: ManualExecutionMode
  api_key_id: number
  mode: 'generate' | 'edit'
  model: string
  prompt: string
  size_mode: ImageSizeMode
  size: string
  custom_size: string
  quality: string
  n: number
  download_image: boolean
  response_format: ImageMonitorResponseFormat
  timeout_seconds: number
  concurrency: number
  input_image_ref?: string
  input_image_type?: string
  input_image_name?: string
}

type ManualPreset = {
  id: string
  name: string
  settings: ManualPresetSettings
  created_at: string
  updated_at: string
}

type ManualHistoryItem = {
  id: string
  run_id: string
  monitor_id: number
  monitor_name: string
  batch_id: string
  batch_size: number
  batch_index: number
  batch_average_elapsed_ms: number
  mode: 'generate' | 'edit'
  execution_mode?: ManualExecutionMode
  gateway_status?: string
  delivery_status?: string
  status: ImageMonitorStatus | 'canceled' | 'observation_lost'
  stage: string
  message: string
  elapsed_ms: number
  started_at: string
  completed_at: string
  model: string
  prompt: string
  size: string
  quality: string
  n: number
  download_image: boolean
  response_format?: ImageMonitorResponseFormat
  input_image_ref?: string
  input_image_type?: string
  input_image_name?: string
  output_image_ref?: string
  output_image_url?: string
  output_artifact_pending?: boolean
  output_artifact_index?: number
  output_artifact_monitor_id?: number
  output_artifact_timeout_seconds?: number
  output_artifact_retry_count?: number
  output_artifact_expires_at?: string
  result?: ImageChannelMonitorResult
}

type ManualInputImage = {
  data: string
  blob?: Blob
  type: string
  name: string
}

type ManualGatewayTestParams = ImageChannelManualTestParams & {
  execution_mode: ManualExecutionMode
  api_key_id?: number
  expected_account_id?: number
  client_run_id: string
}

type ManualPersistedImage = {
  data?: string
  blob?: Blob
  type: string
  name: string
}

type ManualStoredImage = ManualPersistedImage & {
  ref: string
  saved_at: string
}

type ManualArtifactLoadResult = {
  image: ManualPersistedImage | null
  index: number | null
  pending: boolean
}

type NetworkInfoItem = {
  label: string
  value: string
  href?: string
}

type WaterfallSegment = {
  key: string
  label: string
  ms: number
  pct: number
}

type ManualRecordStatus = ImageMonitorStatus | 'running' | 'canceled' | 'observation_lost'
type ManualRecordSource = 'live' | 'history'
type ManualActualResponseKind = '' | 'url' | 'b64Json' | 'dataUrl' | 'urlAndB64Json' | 'dataUrlAndB64Json'

type ManualRecordColumnKey =
  | 'started_at'
  | 'monitor'
  | 'status'
  | 'batch'
  | 'mode'
  | 'model'
  | 'size'
  | 'elapsed'
  | 'api_total'
  | 'image_download'
  | 'actual_response'
  | 'image_info'
  | 'exit_ip'
  | 'output'

type ManualRecordColumn = {
  key: ManualRecordColumnKey
  label: string
}

type ManualRecordEntry = {
  id: string
  run_id: string
  source: ManualRecordSource
  monitor_id: number
  monitor_name: string
  batch_id: string
  batch_size: number
  batch_index: number
  batch_average_elapsed_ms: number
  mode: 'generate' | 'edit'
  execution_mode: ManualExecutionMode
  gateway_status: string
  delivery_status: string
  status: ManualRecordStatus
  stage: string
  message: string
  elapsed_ms: number
  started_at: string
  completed_at: string
  model: string
  prompt: string
  size: string
  quality: string
  n: number
  download_image: boolean
  response_format?: ImageMonitorResponseFormat
  result?: ImageChannelMonitorResult
  liveItem?: ManualResultItem
  historyItem?: ManualHistoryItem
  inputPreview?: string
  outputPreview?: string
  outputUrl?: string
}

const monitors = ref<ImageChannelMonitor[]>([])
const loading = ref(false)
const saving = ref(false)
const runningId = ref<number | null>(null)
const searchQuery = ref('')
const sourceFilter = ref<ImageMonitorSourceType | ''>('')
const enabledFilter = ref<'' | 'true' | 'false'>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const showDialog = ref(false)
const editing = ref<ImageChannelMonitor | null>(null)
const showDeleteDialog = ref(false)
const deleting = ref<ImageChannelMonitor | null>(null)
const lastRunResult = ref<ImageChannelMonitorResult | null>(null)
const showHistoryDialog = ref(false)
const historyItems = ref<ImageChannelMonitorHistoryItem[]>([])
const statusDialogTarget = ref<ImageChannelMonitor | null>(null)
const accountOptions = ref<Account[]>([])
const accountsLoading = ref(false)
const proxyOptions = ref<Proxy[]>([])
const proxiesLoading = ref(false)
const runtimeStatuses = ref<Record<number, ImageChannelMonitorRuntimeStatus>>({})
const nowMs = ref(Date.now())
const activePanel = ref<ImageMonitorPanel>('monitors')
const manualTargets = ref<ImageChannelMonitor[]>([])
const manualTargetsLoading = ref(false)
const manualSelectedIds = ref<number[]>([])
const manualRunning = ref(false)
const manualResults = ref<Record<string, ManualResultItem>>({})
const manualHistory = ref<ManualHistoryItem[]>([])
const manualHistoryInputPreviews = ref<Record<string, string>>({})
const manualHistoryOutputPreviews = ref<Record<string, string>>({})
const manualPresets = ref<ManualPreset[]>([])
const manualPresetSelectedId = ref('')
const manualPresetName = ref('')
const manualInputImages = ref<ManualInputImage[]>([])
const manualInputImage = computed<ManualInputImage | null>({
  get: () => manualInputImages.value[0] || null,
  set: (value) => {
    manualInputImages.value = value ? [value] : []
  },
})
const manualImageInputKey = ref(0)
const manualExecutionMode = ref<ManualExecutionMode>('gateway_account')
const manualAPIKeyID = ref(0)
const manualAPIKeys = ref<SimpleApiKey[]>([])
const manualAPIKeysLoading = ref(false)
const manualConfigCollapsed = ref(false)
const manualTargetSearch = ref('')
const manualBatchDismissed = ref(false)
const manualActiveBatchID = ref('')
const showManualPresetSaveDialog = ref(false)
const fieldsDetails = ref<HTMLDetailsElement | null>(null)
const selectedManualRecord = ref<ManualRecordEntry | null>(null)
const manualRecordSearch = ref('')
const manualRecordStatusFilter = ref<ManualRecordStatus | ''>('')
const manualRecordModeFilter = ref<'' | 'generate' | 'edit'>('')
const manualRecordMonitorFilter = ref<number | ''>('')
const manualRecordSort = ref<'newest' | 'oldest'>('newest')
const manualRunOutputPreviews = ref<Record<string, string>>({})
const manualVisibleColumns = ref<ManualRecordColumnKey[]>([
  'started_at',
  'monitor',
  'status',
  'batch',
  'elapsed',
  'api_total',
  'image_download',
  'actual_response',
  'image_info',
  'exit_ip',
  'output',
])

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let statusPollTimer: number | null = null
let clockTimer: number | null = null
let manualRunSeq = 0
let manualViewDisposed = false
let manualStartRetryAbortController: AbortController | null = null
const manualCanceledRunSeqs = new Set<number>()
const manualHistoryPendingRunIDs = new Set<string>()
const manualArtifactPendingRunIDs = new Set<string>()
const manualTerminalPendingRunIDs = new Set<string>()
const manualOutputObjectURLs = new Map<string, string>()
const manualPendingOutputImages = new Map<string, ManualPersistedImage>()
const manualInputObjectURLs = new Set<string>()
const manualArtifactRecoveryTimers = new Map<string, number>()

const manualPresetStorageKey = 'sub2api:image-channel-monitor:manual-presets:v1'
const manualHistoryStorageKey = 'sub2api:image-channel-monitor:manual-history:v1'
const manualImageDBName = 'sub2api-image-channel-monitor'
const manualImageStoreName = 'manual-images'
const defaultStandardSize = '1024x1024'
const manualConcurrencyMin = 1
const manualConcurrencyMax = 20
const manualRunTotalMax = 100

const standardSizeOptions = [
  { labelKey: 'admin.imageChannelMonitor.sizes.square', value: '1024x1024' },
  { labelKey: 'admin.imageChannelMonitor.sizes.landscape', value: '1536x1024' },
  { labelKey: 'admin.imageChannelMonitor.sizes.portrait', value: '1024x1536' },
]

const manualExecutionModeOptions = computed(() => [
  {
    value: 'gateway_account' as const,
    label: t('admin.imageChannelMonitor.manual.executionModes.gatewayAccount'),
  },
  {
    value: 'gateway_group' as const,
    label: t('admin.imageChannelMonitor.manual.executionModes.gatewayGroup'),
  },
  {
    value: 'direct_probe' as const,
    label: t('admin.imageChannelMonitor.manual.executionModes.directProbe'),
  },
])

const form = reactive({
  name: '',
  source_type: 'custom' as ImageMonitorSourceType,
  endpoint: 'https://api.openai.com',
  api_key: '',
  account_id: null as number | null,
  proxy_id: 0,
  model: 'gpt-image-1',
  prompt: 'Generate a simple health-check image with a clean geometric shape.',
  size_mode: 'omit' as ImageSizeMode,
  size: defaultStandardSize,
  custom_size: '',
  quality: 'auto',
  n: 1,
  download_image: true,
  response_format: 'url' as ImageMonitorResponseFormat,
  enabled: true,
  public_visible: false,
  public_name: '',
  interval_seconds: 300,
  timeout_seconds: 300,
})

const manualForm = reactive({
  mode: 'generate' as 'generate' | 'edit',
  model: 'gpt-image-1',
  prompt: 'Generate a simple health-check image with a clean geometric shape.',
  size_mode: 'omit' as ImageSizeMode,
  size: defaultStandardSize,
  custom_size: '',
  quality: 'auto',
  n: 1,
  download_image: true,
  response_format: 'url' as ImageMonitorResponseFormat,
  timeout_seconds: 300,
  concurrency: 1,
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.imageChannelMonitor.columns.name'), sortable: false },
  { key: 'source_type', label: t('admin.imageChannelMonitor.columns.source'), sortable: false },
  { key: 'model', label: t('admin.imageChannelMonitor.columns.model'), sortable: false },
  { key: 'last_checked_at', label: t('admin.imageChannelMonitor.columns.checkedAt'), sortable: false },
  { key: 'enabled', label: t('admin.imageChannelMonitor.columns.enabled'), sortable: false },
  { key: 'actions', label: t('admin.imageChannelMonitor.columns.actions'), sortable: false },
])

const deleteConfirmMessage = computed(() =>
  t('admin.imageChannelMonitor.deleteConfirm', { name: deleting.value?.name || '' })
)

const lastRunPreview = computed(() => {
  if (!lastRunResult.value) return ''
  return lastRunResult.value.returned_image_url || lastRunResult.value.returned_image_data
})

const manualResultList = computed(() =>
  Object.values(manualResults.value).sort((a, b) => {
    const leftBatch = a.batch_id || a.startedAt || ''
    const rightBatch = b.batch_id || b.startedAt || ''
    if (leftBatch !== rightBatch) return leftBatch.localeCompare(rightBatch)
    if (a.batch_index !== b.batch_index) return a.batch_index - b.batch_index
    return a.monitor.id - b.monitor.id
  })
)

const manualSelectableTargets = computed(() => {
  if (manualExecutionMode.value === 'direct_probe') return manualTargets.value
  return manualTargets.value.filter(
    (target) => target.source_type === 'account' && Number(target.account_id) > 0
  )
})

const manualFilteredTargets = computed(() => {
  const query = manualTargetSearch.value.trim().toLowerCase()
  if (!query) return manualSelectableTargets.value
  return manualSelectableTargets.value.filter((target) => {
    const haystack = [
      target.name,
      target.model,
      target.endpoint,
      target.account_name,
      `#${target.id}`,
    ]
    return haystack.some((value) => String(value || '').toLowerCase().includes(query))
  })
})

// Merged size dropdown: single control backing the size_mode/size/custom_size trio.
const manualSizeChoice = computed<string>({
  get() {
    switch (manualForm.size_mode) {
      case 'auto':
        return 'auto'
      case 'custom':
        return 'custom'
      case 'preset':
        return manualForm.size
      default:
        return 'omit'
    }
  },
  set(value: string) {
    if (value === 'omit' || value === 'auto' || value === 'custom') {
      manualForm.size_mode = value as ImageSizeMode
      return
    }
    manualForm.size_mode = 'preset'
    manualForm.size = value
  },
})

const manualResolvedSizeLabel = computed(() =>
  formatSize(resolvedManualSizeFromSettings(currentManualPresetSettings()))
)

const manualNormalizedConcurrency = computed(() =>
  clampManualConcurrency(manualForm.concurrency)
)

const manualPlannedRunCount = computed(() =>
  manualSelectedTargetCount.value * manualNormalizedConcurrency.value
)

const manualSelectedTargetCount = computed(() => {
  const selectable = new Set(manualSelectableTargets.value.map((target) => target.id))
  return manualSelectedIds.value.filter((id) => selectable.has(id)).length
})

const manualRequiredInputImageCount = computed(() => (manualForm.mode === 'edit' ? 1 : 0))

const manualStartDisabled = computed(() => {
  if (manualRunning.value || manualSelectedTargetCount.value === 0) return true
  if (manualExecutionMode.value !== 'direct_probe' && manualAPIKeyID.value <= 0) return true
  if (
    manualForm.mode === 'edit' &&
    manualInputImages.value.length < manualRequiredInputImageCount.value
  ) {
    return true
  }
  return false
})

const manualExecutionModeHint = computed(() =>
  t(`admin.imageChannelMonitor.manual.executionModeHints.${manualExecutionMode.value}`)
)

const manualCTAHint = computed(() =>
  t(`admin.imageChannelMonitor.manual.ctaHints.${manualExecutionMode.value}`, {
    concurrency: manualNormalizedConcurrency.value,
  })
)

const manualConfigSummary = computed(() => [
  manualExecutionModeLabel(manualExecutionMode.value),
  manualForm.mode === 'edit'
    ? t('admin.imageChannelMonitor.manual.edit')
    : t('admin.imageChannelMonitor.manual.generate'),
  manualForm.model || '-',
  manualResolvedSizeLabel.value,
  `n=${manualForm.n}`,
  t('admin.imageChannelMonitor.manual.concurrencySummary', {
    count: manualNormalizedConcurrency.value,
  }),
  responseFormatLabel(manualForm.response_format),
])

const manualBatchStats = computed(() => {
  let running = 0
  let ok = 0
  let failed = 0
  let canceled = 0
  let observationLost = 0
  for (const item of manualResultList.value) {
    if (item.state === 'running') {
      running += 1
      continue
    }
    if (item.state === 'canceled') {
      canceled += 1
      continue
    }
    if (item.state === 'observation_lost') {
      observationLost += 1
      continue
    }
    const status = manualRunResult(item)?.status
    if (item.state === 'done' && (status === 'operational' || status === 'degraded')) {
      ok += 1
    } else {
      failed += 1
    }
  }
  const total = manualResultList.value.length
  return { total, running, done: total - running, ok, failed, canceled, observationLost }
})

const manualShowBatchBanner = computed(
  () => !manualBatchDismissed.value && manualBatchStats.value.total > 0
)

const manualBatchProgress = computed(() => {
  const { total, done } = manualBatchStats.value
  return total > 0 ? Math.round((done / total) * 100) : 0
})

const manualCurrentBatchAverageMs = computed(() => {
  if (!manualActiveBatchID.value) return null
  const average = manualBatchAverageElapsedMs(manualActiveBatchID.value)
  return average > 0 ? average : null
})

const selectedRecordWaterfall = computed(() =>
  manualWaterfallSegments(selectedManualRecord.value?.result)
)

const waterfallSegmentClass: Record<string, string> = {
  apiHeader: 'bg-primary-300 dark:bg-primary-400',
  apiBody: 'bg-primary-500 dark:bg-primary-500',
  imageDownload: 'bg-accent-400 dark:bg-accent-500',
}

const manualRecordColumns = computed<ManualRecordColumn[]>(() => [
  { key: 'started_at', label: t('admin.imageChannelMonitor.manual.columns.startedAt') },
  { key: 'monitor', label: t('admin.imageChannelMonitor.manual.columns.monitor') },
  { key: 'status', label: t('admin.imageChannelMonitor.manual.columns.status') },
  { key: 'batch', label: t('admin.imageChannelMonitor.manual.columns.batch') },
  { key: 'mode', label: t('admin.imageChannelMonitor.manual.columns.mode') },
  { key: 'model', label: t('admin.imageChannelMonitor.manual.columns.model') },
  { key: 'size', label: t('admin.imageChannelMonitor.manual.columns.size') },
  { key: 'elapsed', label: t('admin.imageChannelMonitor.manual.columns.elapsed') },
  { key: 'api_total', label: t('admin.imageChannelMonitor.manual.columns.apiTotal') },
  { key: 'image_download', label: t('admin.imageChannelMonitor.manual.columns.imageDownload') },
  { key: 'actual_response', label: t('admin.imageChannelMonitor.manual.columns.actualResponse') },
  { key: 'image_info', label: t('admin.imageChannelMonitor.manual.columns.imageInfo') },
  { key: 'exit_ip', label: t('admin.imageChannelMonitor.manual.columns.exitIp') },
  { key: 'output', label: t('admin.imageChannelMonitor.manual.columns.output') },
])

const manualVisibleColumnCount = computed(() => manualVisibleColumns.value.length + 1)

const manualRecordMonitorOptions = computed(() => {
  const options = new Map<number, string>()
  manualTargets.value.forEach((target) => options.set(target.id, target.name))
  manualHistory.value.forEach((entry) => options.set(entry.monitor_id, entry.monitor_name))
  manualResultList.value.forEach((item) => options.set(item.monitor.id, item.monitor.name))
  return Array.from(options.entries())
    .map(([id, name]) => ({ id, name }))
    .sort((a, b) => a.name.localeCompare(b.name))
})

const manualTableEntries = computed<ManualRecordEntry[]>(() => {
  const historyRunIDs = new Set(manualHistory.value.map((entry) => entry.run_id).filter(Boolean))
  const liveEntries = manualResultList.value
    .filter((item) => !item.run?.run_id || !historyRunIDs.has(item.run.run_id))
    .map(manualRecordFromLive)
  const historyEntries = manualHistory.value.map(manualRecordFromHistory)
  return [...liveEntries, ...historyEntries].sort(compareManualRecords)
})

const filteredManualTableEntries = computed(() => {
  const query = manualRecordSearch.value.trim().toLowerCase()
  return manualTableEntries.value.filter((entry) => {
    if (manualRecordStatusFilter.value && entry.status !== manualRecordStatusFilter.value) return false
    if (manualRecordModeFilter.value && entry.mode !== manualRecordModeFilter.value) return false
    if (manualRecordMonitorFilter.value && entry.monitor_id !== manualRecordMonitorFilter.value) return false
    if (query && !manualRecordMatchesSearch(entry, query)) return false
    return true
  })
})

function compareManualRecords(a: ManualRecordEntry, b: ManualRecordEntry) {
  const left = new Date(a.started_at).getTime()
  const right = new Date(b.started_at).getTime()
  const leftValue = Number.isFinite(left) ? left : 0
  const rightValue = Number.isFinite(right) ? right : 0
  return manualRecordSort.value === 'oldest' ? leftValue - rightValue : rightValue - leftValue
}

function manualRecordFromLive(item: ManualResultItem): ManualRecordEntry {
  const result = manualRunResult(item)
  const settings = item.settings || currentManualPresetSettings()
  const startedAt = item.run?.started_at || item.startedAt || new Date(nowMs.value).toISOString()
  const completedAt = item.run?.completed_at || item.completedAt || ''
  const endAt = completedAt || (item.state === 'running' ? new Date(nowMs.value).toISOString() : item.run?.updated_at || startedAt)
  const status = manualRecordStatusFromLive(item)
  const stage = item.run?.stage || result?.error_stage || result?.stages?.at(-1)?.stage || ''
  const batchID = item.run?.batch_id || item.batch_id
  const batchSize = item.run?.batch_size || item.batch_size
  const batchIndex = item.run?.batch_index || item.batch_index
  const executionMode = manualRunExecutionMode(item.run, settings.execution_mode)
  return {
    id: item.run?.run_id ? `live-${item.run.run_id}` : `live-${item.recordId}`,
    run_id: item.run?.run_id || '',
    source: 'live',
    monitor_id: item.monitor.id,
    monitor_name:
      executionMode === 'gateway_group'
        ? t('admin.imageChannelMonitor.manual.gatewayGroupScheduledTarget')
        : item.monitor.name,
    batch_id: batchID,
    batch_size: batchSize,
    batch_index: batchIndex,
    batch_average_elapsed_ms: manualBatchAverageElapsedMs(batchID),
    mode: settings.mode,
    execution_mode: executionMode,
    gateway_status: manualRunStatusField(item.run, 'gateway_status'),
    delivery_status: manualRunStatusField(item.run, 'delivery_status'),
    status,
    stage,
    message: item.message || result?.message || '',
    elapsed_ms: elapsedMs(startedAt, endAt),
    started_at: startedAt,
    completed_at: completedAt,
    model: settings.model,
    prompt: settings.prompt,
    size: resolvedManualSizeFromSettings(settings),
    quality: settings.quality,
    n: settings.n,
    download_image: settings.download_image,
    response_format: settings.response_format,
    result,
    liveItem: item,
    inputPreview: item.inputImage?.data || '',
    outputPreview: manualPreview(item),
    outputUrl: result?.returned_image_url || result?.returned_image_data || '',
  }
}

function manualRecordFromHistory(entry: ManualHistoryItem): ManualRecordEntry {
  return {
    id: `history-${entry.id}`,
    run_id: entry.run_id,
    source: 'history',
    monitor_id: entry.monitor_id,
    monitor_name: entry.monitor_name,
    batch_id: entry.batch_id,
    batch_size: entry.batch_size,
    batch_index: entry.batch_index,
    batch_average_elapsed_ms: entry.batch_average_elapsed_ms,
    mode: entry.mode,
    execution_mode: entry.execution_mode || 'direct_probe',
    gateway_status: entry.gateway_status || '',
    delivery_status: entry.delivery_status || '',
    status: entry.status,
    stage: entry.stage || entry.result?.error_stage || entry.result?.stages?.at(-1)?.stage || '',
    message: entry.message || entry.result?.message || '',
    elapsed_ms: entry.elapsed_ms,
    started_at: entry.started_at,
    completed_at: entry.completed_at,
    model: entry.model,
    prompt: entry.prompt,
    size: entry.size,
    quality: entry.quality,
    n: entry.n,
    download_image: entry.download_image,
    response_format: entry.response_format ?? entry.result?.response_format,
    result: entry.result,
    historyItem: entry,
    inputPreview: manualHistoryInputPreview(entry),
    outputPreview: manualHistoryOutputPreview(entry),
    outputUrl: entry.output_image_url || entry.result?.returned_image_url || entry.result?.returned_image_data || '',
  }
}

function manualRecordStatusFromLive(item: ManualResultItem): ManualRecordStatus {
  if (item.state === 'running') return 'running'
  if (item.state === 'canceled') return 'canceled'
  if (item.state === 'observation_lost') return 'observation_lost'
  if (item.state === 'error') return 'error'
  return manualRunResult(item)?.status || 'error'
}

type ManualRunExtendedFields = {
  execution_mode?: ManualExecutionMode
  gateway_status?: string
  delivery_status?: string
  observation_status?: string
  artifacts?: Array<{
    index: number
    content_type?: string
    size?: number
    source?: 'b64_json' | 'url'
  }>
}

function manualRunExtendedFields(run?: ImageChannelManualRunResponse): ManualRunExtendedFields {
  return (run || {}) as ManualRunExtendedFields
}

function manualRunExecutionMode(
  run: ImageChannelManualRunResponse | undefined,
  fallback: ManualExecutionMode = 'gateway_account'
) {
  return normalizeManualExecutionMode(manualRunExtendedFields(run).execution_mode, fallback)
}

function manualRunStatusField(
  run: ImageChannelManualRunResponse | undefined,
  field: 'gateway_status' | 'delivery_status' | 'observation_status'
) {
  return String(manualRunExtendedFields(run)[field] || '').trim()
}

function manualStatusValueText(value: string) {
  const normalized = value.trim()
  if (!normalized) return ''
  const key = `admin.imageChannelMonitor.manual.statusValues.${normalized}`
  const translated = t(key)
  return translated === key ? normalized : translated
}

function manualRecordMatchesSearch(entry: ManualRecordEntry, query: string) {
  const result = entry.result
  const haystack = [
    entry.monitor_name,
    String(entry.monitor_id),
    entry.model,
    entry.prompt,
    entry.size,
    entry.quality,
    entry.message,
    entry.stage,
    entry.run_id,
    entry.batch_id,
    manualRecordActualResponseText(entry),
    result?.exit_ip,
    result?.request_target_url,
    result?.request_target_host,
    result?.request_target_ips?.join(' '),
    result?.image_download_url,
    result?.image_download_host,
    result?.image_download_ips?.join(' '),
  ]
  return haystack.some((value) => String(value || '').toLowerCase().includes(query))
}

function manualColumnVisible(key: ManualRecordColumnKey) {
  return manualVisibleColumns.value.includes(key)
}

function openManualRecordDetail(entry: ManualRecordEntry) {
  selectedManualRecord.value = entry
}

function manualRecordStatusText(entry: ManualRecordEntry | null) {
  if (!entry) return ''
  if (entry.status === 'running') return t('admin.imageChannelMonitor.manual.running')
  if (entry.status === 'canceled') return t('admin.imageChannelMonitor.manual.canceled')
  if (entry.status === 'observation_lost') {
    return t('admin.imageChannelMonitor.manual.observationLost')
  }
  return statusLabel(entry.status)
}

function manualRecordBadgeClass(entry: ManualRecordEntry | null) {
  if (!entry) return ''
  if (entry.status === 'running') {
    return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
  }
  if (entry.status === 'canceled') {
    return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  }
  if (entry.status === 'observation_lost') {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  }
  return statusBadgeClass(entry.status)
}

function manualRecordStageText(entry: ManualRecordEntry | null) {
  if (!entry) return ''
  if (!entry.stage) return entry.message || ''
  return t(`admin.imageChannelMonitor.stages.${entry.stage}`, entry.stage)
}

function manualRecordInputPreview(entry: ManualRecordEntry | null) {
  return entry?.inputPreview || ''
}

function manualRecordOutputPreview(entry: ManualRecordEntry | null) {
  return entry?.outputPreview || entry?.outputUrl || ''
}

function manualRecordOutputHref(entry: ManualRecordEntry | null) {
  return manualRecordOutputPreview(entry)
}

function manualRecordActualResponseKind(entry: ManualRecordEntry | null): ManualActualResponseKind {
  const result = entry?.result
  if (!result) return ''
  const returnedURL = String(result.returned_image_url || '').trim()
  const hasURL = Boolean(result.has_url || returnedURL)
  const hasB64JSON = Boolean(result.has_b64_json)
  const dataURL = hasURL && isDataURL(returnedURL)
  if (dataURL && hasB64JSON) return 'dataUrlAndB64Json'
  if (hasURL && hasB64JSON) return 'urlAndB64Json'
  if (dataURL) return 'dataUrl'
  if (hasURL) return 'url'
  if (hasB64JSON) return 'b64Json'
  return ''
}

function manualRecordActualResponseText(entry: ManualRecordEntry | null) {
  const kind = manualRecordActualResponseKind(entry)
  return kind ? t(`admin.imageChannelMonitor.manual.returnModes.${kind}`) : ''
}

function manualRecordActualResponseTitle(entry: ManualRecordEntry | null) {
  const result = entry?.result
  const actual = manualRecordActualResponseText(entry)
  if (!result || !actual) return ''
  const parts = [
    `${t('admin.imageChannelMonitor.manual.actualResponse')}: ${actual}`,
    `${t('admin.imageChannelMonitor.form.responseFormat')}: ${responseFormatLabel(entry?.response_format)}`,
  ]
  if (result.returned_image_url) {
    parts.push(`${t('admin.imageChannelMonitor.network.imageUrl')}: ${formatReturnedImageURLForDisplay(result.returned_image_url)}`)
  }
  if (result.has_b64_json) {
    parts.push('b64_json: true')
  }
  return parts.join('\n')
}

function manualRecordActualResponseBadgeClass(entry: ManualRecordEntry | null) {
  const kind = manualRecordActualResponseKind(entry)
  if (kind.includes('DataUrl')) {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  }
  if (kind.includes('B64Json')) {
    return 'bg-purple-50 text-purple-700 dark:bg-purple-900/30 dark:text-purple-200'
  }
  if (kind === 'url') {
    return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
  }
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function manualRecordDownloadName(entry: ManualRecordEntry | null) {
  if (!entry) return 'manual-image-test.png'
  const stamp = entry.started_at ? entry.started_at.replace(/[:.]/g, '-') : 'image'
  return `${sanitizeFileName(entry.monitor_name)}-${stamp}.png`
}

function sanitizeFileName(value: string) {
  return value.trim().replace(/[\\/:*?"<>|]+/g, '-').replace(/\s+/g, '-').slice(0, 80) || 'manual-image-test'
}

function shortManualBatchID(batchID: string) {
  const normalized = batchID.trim()
  if (!normalized) return '-'
  return normalized.length > 10 ? normalized.slice(-10) : normalized
}

function loadManualPresets() {
  try {
    const raw = window.localStorage.getItem(manualPresetStorageKey)
    const parsed = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) {
      manualPresets.value = []
      return
    }
    manualPresets.value = parsed
      .map(normalizeManualPreset)
      .filter((preset): preset is ManualPreset => Boolean(preset))
      .slice(0, 50)
  } catch {
    manualPresets.value = []
  }
}

function persistManualPresets() {
  window.localStorage.setItem(manualPresetStorageKey, JSON.stringify(manualPresets.value))
}

function normalizeManualPreset(raw: unknown): ManualPreset | null {
  if (!raw || typeof raw !== 'object') return null
  const source = raw as Partial<ManualPreset>
  const id = String(source.id || '').trim()
  const name = String(source.name || '').trim()
  if (!id || !name) return null
  const now = new Date().toISOString()
  return {
    id,
    name,
    settings: normalizeManualPresetSettings(source.settings),
    created_at: typeof source.created_at === 'string' && source.created_at ? source.created_at : now,
    updated_at: typeof source.updated_at === 'string' && source.updated_at ? source.updated_at : now,
  }
}

function normalizeManualPresetSettings(
  raw?: Partial<ManualPresetSettings>
): ManualPresetSettings {
  const sizeMode = normalizeImageSizeMode(raw?.size_mode)
  return {
    execution_mode: normalizeManualExecutionMode(raw?.execution_mode, 'direct_probe'),
    api_key_id: clampInt(raw?.api_key_id, 0, 0, Number.MAX_SAFE_INTEGER),
    mode: raw?.mode === 'edit' ? 'edit' : 'generate',
    model: String(raw?.model || 'gpt-image-1').trim() || 'gpt-image-1',
    prompt:
      String(raw?.prompt || 'Generate a simple health-check image with a clean geometric shape.').trim() ||
      'Generate a simple health-check image with a clean geometric shape.',
    size_mode: sizeMode,
    size: String(raw?.size || defaultStandardSize).trim() || defaultStandardSize,
    custom_size: String(raw?.custom_size || '').trim(),
    quality: String(raw?.quality || 'auto').trim() || 'auto',
    n: clampInt(raw?.n, 1, 1, 10),
    download_image: raw?.download_image !== false,
    response_format: normalizeImageResponseFormat(raw?.response_format),
    timeout_seconds: clampInt(raw?.timeout_seconds, 300, 30, 600),
    concurrency: clampManualConcurrency(raw?.concurrency),
    input_image_ref: typeof raw?.input_image_ref === 'string' ? raw.input_image_ref : '',
    input_image_type: typeof raw?.input_image_type === 'string' ? raw.input_image_type : '',
    input_image_name: typeof raw?.input_image_name === 'string' ? raw.input_image_name : '',
  }
}

function normalizeManualExecutionMode(
  value: unknown,
  fallback: ManualExecutionMode = 'gateway_account'
): ManualExecutionMode {
  if (value === 'gateway_account' || value === 'gateway_group' || value === 'direct_probe') {
    return value
  }
  return fallback
}

function manualExecutionModeLabel(value: ManualExecutionMode) {
  const suffix =
    value === 'gateway_account'
      ? 'gatewayAccount'
      : value === 'gateway_group'
        ? 'gatewayGroup'
        : 'directProbe'
  return t(`admin.imageChannelMonitor.manual.executionModes.${suffix}`)
}

function normalizeImageSizeMode(value: unknown): ImageSizeMode {
  if (value === 'auto' || value === 'preset' || value === 'custom') {
    return value
  }
  return 'omit'
}

// 旧数据(preset/历史)无该字段时回落 'url',与后端历史默认语义一致。
function normalizeImageResponseFormat(value: unknown): ImageMonitorResponseFormat {
  if (value === '' || value === 'b64_json') return value
  return 'url'
}

function responseFormatLabel(value: ImageMonitorResponseFormat | undefined): string {
  if (value === 'url') return 'URL'
  if (value === 'b64_json') return 'Base64'
  if (value === '') return t('admin.imageChannelMonitor.form.responseFormatOmit')
  return '-'
}

function clampInt(value: unknown, fallback: number, min: number, max: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  return Math.min(max, Math.max(min, Math.trunc(parsed)))
}

function clampManualConcurrency(value: unknown) {
  return clampInt(value, 1, manualConcurrencyMin, manualConcurrencyMax)
}

function currentManualPresetSettings(): ManualPresetSettings {
  return normalizeManualPresetSettings({
    execution_mode: manualExecutionMode.value,
    api_key_id: manualAPIKeyID.value,
    mode: manualForm.mode,
    model: manualForm.model,
    prompt: manualForm.prompt,
    size_mode: manualForm.size_mode,
    size: manualForm.size,
    custom_size: manualForm.custom_size,
    quality: manualForm.quality,
    n: manualForm.n,
    download_image: manualForm.download_image,
    response_format: manualForm.response_format,
    timeout_seconds: manualForm.timeout_seconds,
    concurrency: manualForm.concurrency,
  })
}

function applyManualPresetSettings(settings: ManualPresetSettings) {
  const normalized = normalizeManualPresetSettings(settings)
  setManualExecutionMode(normalized.execution_mode)
  manualAPIKeyID.value = normalized.api_key_id
  Object.assign(manualForm, {
    mode: normalized.mode,
    model: normalized.model,
    prompt: normalized.prompt,
    size_mode: normalized.size_mode,
    size: normalized.size,
    custom_size: normalized.custom_size,
    quality: normalized.quality,
    n: normalized.n,
    download_image: normalized.download_image,
    response_format: normalized.response_format,
    timeout_seconds: normalized.timeout_seconds,
    concurrency: normalized.concurrency,
  })
}

async function handleManualPresetSelect() {
  const preset = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
  if (!preset) {
    manualPresetName.value = ''
	revokeManualInputObjectURLs()
    manualInputImage.value = null
    return
  }
  manualPresetName.value = preset.name
  applyManualPresetSettings(preset.settings)
  const selectedID = preset.id
  if (preset.settings.input_image_ref) {
    try {
      const stored = await loadManualStoredImage(preset.settings.input_image_ref)
      if (manualPresetSelectedId.value !== selectedID) return
		revokeManualInputObjectURLs()
		const preview = stored?.data || (stored?.blob?.size ? URL.createObjectURL(stored.blob) : '')
		if (preview.startsWith('blob:')) manualInputObjectURLs.add(preview)
      manualInputImage.value = stored?.data || stored?.blob?.size
        ? {
			data: preview,
            blob: stored.blob,
            type: stored.type,
            name: stored.name,
          }
        : null
    } catch {
      if (manualPresetSelectedId.value === selectedID) {
		revokeManualInputObjectURLs()
        manualInputImage.value = null
      }
    }
  } else {
	revokeManualInputObjectURLs()
    manualInputImage.value = null
  }
}

function openManualPresetSaveDialog() {
  const existing = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
  manualPresetName.value = existing?.name || manualPresetName.value
  showManualPresetSaveDialog.value = true
}

async function saveManualPreset() {
  const name = manualPresetName.value.trim()
  if (!name) {
    appStore.showError(t('admin.imageChannelMonitor.manual.presetNameRequired'))
    return
  }
  try {
    const now = new Date().toISOString()
    const existing = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
    const settings = currentManualPresetSettings()
    if (manualForm.mode === 'edit' && (manualInputImage.value?.data || manualInputImage.value?.blob?.size)) {
      settings.input_image_ref = await saveManualStoredImage(manualInputImage.value, 'preset-input')
      settings.input_image_type = manualInputImage.value.type
      settings.input_image_name = manualInputImage.value.name
    }
    if (
      existing?.settings.input_image_ref &&
      existing.settings.input_image_ref !== settings.input_image_ref
    ) {
      void deleteManualStoredImage(existing.settings.input_image_ref).catch(() => {})
    }
    const saved: ManualPreset = {
      id: existing?.id || newManualPresetID(),
      name,
      settings,
      created_at: existing?.created_at || now,
      updated_at: now,
    }
    const nextPresets = [
      saved,
      ...manualPresets.value.filter((item) => item.id !== saved.id),
    ]
    const droppedPresets = nextPresets.slice(50)
    manualPresets.value = nextPresets.slice(0, 50)
    void Promise.allSettled(droppedPresets.map((preset) => deleteManualStoredImage(preset.settings.input_image_ref)))
    manualPresetSelectedId.value = saved.id
    manualPresetName.value = saved.name
    persistManualPresets()
    showManualPresetSaveDialog.value = false
    appStore.showSuccess(t('admin.imageChannelMonitor.manual.presetSaved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.presetSaveFailed')))
  }
}

async function deleteManualPreset() {
  const id = manualPresetSelectedId.value
  if (!id) return
  const preset = manualPresets.value.find((item) => item.id === id)
  void deleteManualStoredImage(preset?.settings.input_image_ref).catch(() => {})
  manualPresets.value = manualPresets.value.filter((item) => item.id !== id)
  manualPresetSelectedId.value = ''
  manualPresetName.value = ''
  persistManualPresets()
  appStore.showSuccess(t('admin.imageChannelMonitor.manual.presetDeleted'))
}

function newManualPresetID() {
  if (window.crypto?.randomUUID) {
    return window.crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function newManualBatchID() {
  return `cmb-${newManualPresetID()}`
}

function openManualImageDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = window.indexedDB.open(manualImageDBName, 1)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(manualImageStoreName)) {
        db.createObjectStore(manualImageStoreName, { keyPath: 'ref' })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

async function saveManualStoredImage(
  image: ManualPersistedImage,
  scope: 'preset-input' | 'history-input' | 'history-output'
) {
  const db = await openManualImageDB()
  const ref = `${scope}:${newManualPresetID()}`
  const stored: ManualStoredImage = {
    ref,
    type: image.type,
    name: image.name,
    saved_at: new Date().toISOString(),
    ...(image.blob ? { blob: image.blob } : { data: image.data || '' }),
  }
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(manualImageStoreName, 'readwrite')
    tx.objectStore(manualImageStoreName).put(stored)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
  return ref
}

async function loadManualStoredImage(ref?: string) {
  if (!ref) return null
  const db = await openManualImageDB()
  const stored = await new Promise<ManualStoredImage | null>((resolve, reject) => {
    const tx = db.transaction(manualImageStoreName, 'readonly')
    const request = tx.objectStore(manualImageStoreName).get(ref)
    request.onsuccess = () => resolve((request.result as ManualStoredImage | undefined) || null)
    request.onerror = () => reject(request.error)
  })
  db.close()
  return stored
}

async function deleteManualStoredImage(ref?: string) {
  if (!ref) return
  const db = await openManualImageDB()
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(manualImageStoreName, 'readwrite')
    tx.objectStore(manualImageStoreName).delete(ref)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
}

async function trySaveManualStoredImage(
  image: ManualPersistedImage | null | undefined,
  scope: 'preset-input' | 'history-input' | 'history-output'
) {
  if (!image || (!image.data && !image.blob?.size)) return ''
  try {
    return await saveManualStoredImage(image, scope)
  } catch {
    return ''
  }
}

function loadManualHistory() {
  try {
    const raw = window.localStorage.getItem(manualHistoryStorageKey)
    const parsed = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) {
      manualHistory.value = []
      return
    }
    manualHistory.value = parsed
      .map(normalizeManualHistoryItem)
      .filter((item): item is ManualHistoryItem => Boolean(item))
      .slice(0, 50)
    void hydrateManualHistoryImages()
  } catch {
    manualHistory.value = []
  }
}

function persistManualHistory() {
  window.localStorage.setItem(manualHistoryStorageKey, JSON.stringify(manualHistory.value))
}

function normalizeManualHistoryItem(raw: unknown): ManualHistoryItem | null {
  if (!raw || typeof raw !== 'object') return null
  const source = raw as Partial<ManualHistoryItem>
  const id = String(source.id || '').trim()
  const runID = String(source.run_id || '').trim()
  const monitorName = String(source.monitor_name || '').trim()
  if (!id || !runID || !monitorName) return null
  const completedAt = String(source.completed_at || '').trim()
  const startedAt = String(source.started_at || completedAt).trim()
  return {
    id,
    run_id: runID,
    monitor_id: clampInt(source.monitor_id, 0, 0, Number.MAX_SAFE_INTEGER),
    monitor_name: monitorName,
    batch_id: typeof source.batch_id === 'string' ? source.batch_id.trim() : '',
    batch_size: clampInt(source.batch_size, 0, 0, Number.MAX_SAFE_INTEGER),
    batch_index: clampInt(source.batch_index, 0, 0, Number.MAX_SAFE_INTEGER),
    batch_average_elapsed_ms: clampInt(
      source.batch_average_elapsed_ms,
      0,
      0,
      Number.MAX_SAFE_INTEGER
    ),
    mode: source.mode === 'edit' ? 'edit' : 'generate',
    execution_mode: normalizeManualExecutionMode(source.execution_mode, 'direct_probe'),
    gateway_status: String(source.gateway_status || '').trim(),
    delivery_status: String(source.delivery_status || '').trim(),
    status: normalizeManualHistoryStatus(source.status),
    stage: String(source.stage || '').trim(),
    message: String(source.message || '').trim(),
    elapsed_ms: clampInt(source.elapsed_ms, elapsedMs(startedAt, completedAt), 0, Number.MAX_SAFE_INTEGER),
    started_at: startedAt,
    completed_at: completedAt,
    model: String(source.model || '').trim(),
    prompt: String(source.prompt || '').trim(),
    size: String(source.size || '').trim(),
    quality: String(source.quality || '').trim(),
    n: clampInt(source.n, 1, 1, 10),
    download_image: source.download_image !== false,
    response_format:
      typeof source.response_format === 'string'
        ? normalizeImageResponseFormat(source.response_format)
        : undefined,
    input_image_ref: typeof source.input_image_ref === 'string' ? source.input_image_ref : '',
    input_image_type: typeof source.input_image_type === 'string' ? source.input_image_type : '',
    input_image_name: typeof source.input_image_name === 'string' ? source.input_image_name : '',
    output_image_ref: typeof source.output_image_ref === 'string' ? source.output_image_ref : '',
    output_image_url: typeof source.output_image_url === 'string' ? source.output_image_url : '',
    output_artifact_pending: source.output_artifact_pending === true,
    output_artifact_index: Number.isSafeInteger(source.output_artifact_index)
      ? Number(source.output_artifact_index)
      : undefined,
    output_artifact_monitor_id: Number.isSafeInteger(source.output_artifact_monitor_id)
      ? Number(source.output_artifact_monitor_id)
      : undefined,
    output_artifact_timeout_seconds: Number.isSafeInteger(source.output_artifact_timeout_seconds)
      ? Number(source.output_artifact_timeout_seconds)
      : undefined,
    output_artifact_retry_count: Number.isSafeInteger(source.output_artifact_retry_count)
      ? Number(source.output_artifact_retry_count)
      : 0,
    output_artifact_expires_at:
      typeof source.output_artifact_expires_at === 'string'
        ? source.output_artifact_expires_at
        : manualArtifactRecoveryExpiresAt(completedAt),
    result: compactManualHistoryResult(source.result),
  }
}

function normalizeManualHistoryStatus(
  value: unknown
): ImageMonitorStatus | 'canceled' | 'observation_lost' {
  if (
    value === 'operational' ||
    value === 'degraded' ||
    value === 'failed' ||
    value === 'error' ||
    value === 'canceled' ||
    value === 'observation_lost'
  ) {
    return value
  }
  return 'error'
}

async function appendManualHistoryFromRun(
  target: ImageChannelMonitor,
  item: ManualResultItem,
  artifactLoad?: ManualArtifactLoadResult
) {
  const run = item.run
  if (!run?.run_id) return
  if (manualHistory.value.some((entry) => entry.run_id === run.run_id)) return
  if (manualHistoryPendingRunIDs.has(run.run_id)) return
  manualHistoryPendingRunIDs.add(run.run_id)
  try {
    const settings = item.settings || currentManualPresetSettings()
    const executionMode = manualRunExecutionMode(run, settings.execution_mode)
    const completedAt = run.completed_at || new Date().toISOString()
    const result = compactManualHistoryResult(run.result)
    const outputImage =
      artifactLoad?.image ||
      (run.result?.returned_image_data
        ? dataURLToManualInputImage(run.result.returned_image_data, 'generated-image')
        : null)
    const inputImageRef = await trySaveManualStoredImage(item.inputImage, 'history-input')
    const outputImageRef = await trySaveManualStoredImage(outputImage, 'history-output')
    if (outputImageRef) {
      manualPendingOutputImages.delete(run.run_id)
    }
    const entry: ManualHistoryItem = {
      id: run.run_id,
      run_id: run.run_id,
      monitor_id: run.monitor?.id || target.id,
      monitor_name:
        executionMode === 'gateway_group'
          ? t('admin.imageChannelMonitor.manual.gatewayGroupScheduledTarget')
          : run.monitor?.name || target.name,
      batch_id: run.batch_id || item.batch_id,
      batch_size: run.batch_size || item.batch_size,
      batch_index: run.batch_index || item.batch_index,
      batch_average_elapsed_ms: 0,
      mode: run.mode,
      execution_mode: executionMode,
      gateway_status: manualRunStatusField(run, 'gateway_status'),
      delivery_status: manualRunStatusField(run, 'delivery_status'),
      status:
        item.state === 'observation_lost'
          ? 'observation_lost'
          : run.canceled
            ? 'canceled'
            : result?.status || 'error',
      stage: run.stage || result?.error_stage || '',
      message: run.message || result?.message || '',
      elapsed_ms: elapsedMs(run.started_at, completedAt),
      started_at: run.started_at,
      completed_at: completedAt,
      model: settings.model,
      prompt: settings.prompt,
      size: resolvedManualSizeFromSettings(settings),
      quality: settings.quality,
      n: settings.n,
      download_image: settings.download_image,
      response_format: settings.response_format,
      input_image_ref: inputImageRef,
      input_image_type: item.inputImage?.type || '',
      input_image_name: item.inputImage?.name || '',
      output_image_ref: outputImageRef,
      output_image_url: run.result?.returned_image_url || '',
      output_artifact_pending:
        !outputImageRef &&
        (artifactLoad?.pending === true || Boolean(artifactLoad?.image && artifactLoad.index !== null)),
      output_artifact_index: artifactLoad?.index ?? undefined,
      output_artifact_monitor_id: run.monitor?.id || target.id,
      output_artifact_timeout_seconds: settings.timeout_seconds,
      output_artifact_retry_count: 0,
      output_artifact_expires_at: manualArtifactRecoveryExpiresAt(completedAt),
      result,
    }
    const nextHistory = [
      entry,
      ...manualHistory.value.filter((history) => history.run_id !== run.run_id),
    ]
    const droppedHistory = nextHistory.slice(50)
    manualHistory.value = nextHistory.slice(0, 50)
    refreshManualHistoryBatchAverage(entry.batch_id)
    persistManualHistory()
    void Promise.allSettled(
      droppedHistory.flatMap((history) => [
        deleteManualStoredImage(history.input_image_ref),
        deleteManualStoredImage(history.output_image_ref),
      ])
    )
    droppedHistory.forEach((history) => {
      cancelManualArtifactRecovery(history.run_id)
      revokeManualRunObjectURL(history.run_id)
    })
    await hydrateManualHistoryImages()
    if (entry.output_artifact_pending) {
      scheduleManualArtifactRecovery(entry.run_id)
    }
  } finally {
    manualHistoryPendingRunIDs.delete(run.run_id)
  }
}

function manualRecordBatchAverage(entry: ManualRecordEntry) {
  if (!entry.batch_id) return 0
  return manualBatchAverageElapsedMs(entry.batch_id) || entry.batch_average_elapsed_ms || entry.elapsed_ms
}

function manualBatchAverageElapsedMs(batchID: string) {
  const normalized = batchID.trim()
  if (!normalized) return 0
  const historyRunIDs = new Set<string>()
  const elapsedValues: number[] = []
  for (const entry of manualHistory.value) {
    if (entry.batch_id !== normalized) continue
    historyRunIDs.add(entry.run_id)
    elapsedValues.push(entry.elapsed_ms)
  }
  for (const item of manualResultList.value) {
    const runID = item.run?.run_id || ''
    if (runID && historyRunIDs.has(runID)) continue
    const itemBatchID = item.run?.batch_id || item.batch_id
    if (itemBatchID !== normalized) continue
    elapsedValues.push(manualResultElapsedMs(item))
  }
  return averageElapsedMs(elapsedValues)
}

function manualResultElapsedMs(item: ManualResultItem) {
  const startedAt = item.run?.started_at || item.startedAt || ''
  const completedAt =
    item.run?.completed_at ||
    item.completedAt ||
    (item.state === 'running' ? new Date(nowMs.value).toISOString() : item.run?.updated_at || '')
  return elapsedMs(startedAt, completedAt || startedAt)
}

function averageElapsedMs(values: number[]) {
  const valid = values.filter((value) => Number.isFinite(value) && value >= 0)
  if (valid.length === 0) return 0
  return Math.round(valid.reduce((sum, value) => sum + value, 0) / valid.length)
}

function refreshManualHistoryBatchAverage(batchID: string) {
  const normalized = batchID.trim()
  if (!normalized) return
  const average = averageElapsedMs(
    manualHistory.value
      .filter((entry) => entry.batch_id === normalized)
      .map((entry) => entry.elapsed_ms)
  )
  manualHistory.value = manualHistory.value.map((entry) =>
    entry.batch_id === normalized
      ? {
          ...entry,
          batch_average_elapsed_ms: average,
        }
      : entry
  )
}

function compactManualHistoryResult(
  result?: ImageChannelMonitorResult
): ImageChannelMonitorResult | undefined {
  if (!result) return undefined
  return {
    ...result,
    returned_image_data: '',
  }
}

async function hydrateManualHistoryImages() {
  const inputPreviews: Record<string, string> = {}
  const outputPreviews: Record<string, string> = {}
  await Promise.allSettled(
    manualHistory.value.map(async (entry) => {
      const [input, output] = await Promise.all([
        loadManualStoredImage(entry.input_image_ref),
        loadManualStoredImage(entry.output_image_ref),
      ])
      const inputPreview = manualStoredImagePreview(entry.run_id, 'input', input)
      const outputPreview = manualStoredImagePreview(entry.run_id, 'output', output)
      if (inputPreview) {
        inputPreviews[entry.id] = inputPreview
      }
      if (outputPreview) {
        outputPreviews[entry.id] = outputPreview
      }
      if (entry.output_artifact_pending) {
        scheduleManualArtifactRecovery(entry.run_id)
      }
    })
  )
  manualHistoryInputPreviews.value = inputPreviews
  manualHistoryOutputPreviews.value = outputPreviews
}

function manualStoredImagePreview(
  runID: string,
  role: 'input' | 'output',
  image: ManualStoredImage | null
) {
  if (!image) return ''
  if (image.blob?.size && typeof URL.createObjectURL === 'function') {
    const key = `${runID}:${role}`
    const previous = manualOutputObjectURLs.get(key)
    if (previous) URL.revokeObjectURL(previous)
    const objectURL = URL.createObjectURL(image.blob)
    manualOutputObjectURLs.set(key, objectURL)
    return objectURL
  }
  return image.data || ''
}

function scheduleManualArtifactRecovery(runID: string) {
  if (manualViewDisposed) return
  const entry = manualHistory.value.find((history) => history.run_id === runID)
  if (!entry?.output_artifact_pending || manualArtifactRecoveryTimers.has(runID)) return
  const remainingMs = manualArtifactRecoveryRemainingMs(entry)
  if (remainingMs <= 0) {
    finishManualArtifactRecovery(runID)
    return
  }
  const attempts = entry.output_artifact_retry_count || 0
  const timer = window.setTimeout(
    () => {
      manualArtifactRecoveryTimers.delete(runID)
      void recoverManualHistoryArtifact(runID)
    },
    Math.min(remainingMs, 12_000, 1_500 * 2 ** Math.min(attempts, 10))
  )
  manualArtifactRecoveryTimers.set(runID, timer)
}

function cancelManualArtifactRecovery(runID: string) {
  const timer = manualArtifactRecoveryTimers.get(runID)
  if (timer !== undefined) {
    window.clearTimeout(timer)
    manualArtifactRecoveryTimers.delete(runID)
  }
}

async function recoverManualHistoryArtifact(runID: string) {
  if (manualViewDisposed) return
  const entry = manualHistory.value.find((history) => history.run_id === runID)
  if (!entry?.output_artifact_pending) return
  if (manualArtifactRecoveryRemainingMs(entry) <= 0) {
    finishManualArtifactRecovery(runID)
    return
  }
  const artifactIndex = Number(entry.output_artifact_index)
  const targetID = entry.output_artifact_monitor_id || entry.monitor_id
  if (!Number.isSafeInteger(artifactIndex) || artifactIndex < 0 || targetID <= 0) {
    finishManualArtifactRecovery(runID)
    return
  }

  const attempt = (entry.output_artifact_retry_count || 0) + 1
  updateManualArtifactRecovery(runID, { output_artifact_retry_count: attempt })
  let storedImage = manualPendingOutputImages.get(runID)
  try {
    if (!storedImage) {
      const blob = await adminAPI.imageChannelMonitor.getManualTestImage(
        targetID,
        runID,
        artifactIndex,
        { timeoutSeconds: entry.output_artifact_timeout_seconds || manualForm.timeout_seconds }
      )
      if (manualViewDisposed) return
      if (!blob.size) throw { status: 0, code: 'EMPTY_ARTIFACT' }
      storedImage = manualPersistedBlob(blob, `generated-image-${runID}`)
      manualPendingOutputImages.set(runID, storedImage)
      setManualRunOutputPreview(runID, storedImage)
    }
  } catch (error: unknown) {
    if (manualViewDisposed) return
    if (isManualRetryableRequestError(error) && manualArtifactRecoveryRemainingMs(entry) > 0) {
      scheduleManualArtifactRecovery(runID)
      return
    }
    finishManualArtifactRecovery(runID)
    return
  }

  try {
    const outputImageRef = await saveManualStoredImage(storedImage, 'history-output')
    manualPendingOutputImages.delete(runID)
    updateManualArtifactRecovery(runID, {
      output_image_ref: outputImageRef,
      output_artifact_pending: false,
      output_artifact_retry_count: attempt,
    })
    const objectURL = manualStoredImagePreview(runID, 'output', {
      ref: outputImageRef,
      ...storedImage,
      saved_at: new Date().toISOString(),
    })
    if (objectURL) {
      manualHistoryOutputPreviews.value = {
        ...manualHistoryOutputPreviews.value,
        [runID]: objectURL,
      }
      manualRunOutputPreviews.value = {
        ...manualRunOutputPreviews.value,
        [runID]: objectURL,
      }
    }
    clearManualArtifactWarning(runID)
  } catch {
    if (manualViewDisposed) return
    if (manualArtifactRecoveryRemainingMs(entry) > 0) {
      scheduleManualArtifactRecovery(runID)
      return
    }
    finishManualArtifactRecovery(runID)
  }
}

function manualArtifactRecoveryRemainingMs(entry: ManualHistoryItem) {
  const expiresAt =
    entry.output_artifact_expires_at || manualArtifactRecoveryExpiresAt(entry.completed_at)
  const expiresAtMs = Date.parse(expiresAt)
  if (!Number.isFinite(expiresAtMs)) return 0
  return Math.max(0, expiresAtMs - Date.now())
}

function updateManualArtifactRecovery(runID: string, patch: Partial<ManualHistoryItem>) {
  manualHistory.value = manualHistory.value.map((history) =>
    history.run_id === runID ? { ...history, ...patch } : history
  )
  persistManualHistory()
}

function finishManualArtifactRecovery(runID: string) {
  manualPendingOutputImages.delete(runID)
  updateManualArtifactRecovery(runID, { output_artifact_pending: false })
  cancelManualArtifactRecovery(runID)
}

function retryManualArtifact(runID: string) {
  cancelManualArtifactRecovery(runID)
  updateManualArtifactRecovery(runID, {
    output_artifact_pending: true,
    output_artifact_retry_count: 0,
  })
  void recoverManualHistoryArtifact(runID)
}

function clearManualArtifactWarning(runID: string) {
  const record = Object.entries(manualResults.value).find(([, item]) => item.run?.run_id === runID)
  if (!record) return
  const [recordID, item] = record
  setManualResult(recordID, { ...item, artifactWarning: undefined })
}

function dataURLToManualInputImage(dataURL: string, fallbackName: string): ManualInputImage | null {
  if (!dataURL.startsWith('data:')) return null
  const match = /^data:([^;,]+).*?,/.exec(dataURL)
  const type = match?.[1] || 'image/png'
  const ext = type.split('/')[1] || 'png'
  return {
    data: dataURL,
    type,
    name: `${fallbackName}.${ext}`,
  }
}

async function clearManualHistory() {
  await Promise.allSettled(
    manualHistory.value.flatMap((entry) => [
      deleteManualStoredImage(entry.input_image_ref),
      deleteManualStoredImage(entry.output_image_ref),
    ])
  )
  manualHistory.value = []
  manualPendingOutputImages.clear()
  for (const runID of manualOutputObjectURLs.keys()) {
    revokeManualRunObjectURL(runID)
  }
  for (const runID of manualArtifactRecoveryTimers.keys()) {
    cancelManualArtifactRecovery(runID)
  }
  manualHistoryInputPreviews.value = {}
  manualHistoryOutputPreviews.value = {}
  if (selectedManualRecord.value?.source === 'history') {
    selectedManualRecord.value = null
  }
  persistManualHistory()
}

function revokeManualRunObjectURL(runID: string) {
  revokeManualObjectURLsForRun(manualOutputObjectURLs, runID)
  if (manualRunOutputPreviews.value[runID]) {
    const next = { ...manualRunOutputPreviews.value }
    delete next[runID]
    manualRunOutputPreviews.value = next
  }
}

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const params: ImageChannelMonitorListParams = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (sourceFilter.value) params.source_type = sourceFilter.value
    if (enabledFilter.value === 'true') params.enabled = true
    if (enabledFilter.value === 'false') params.enabled = false
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()
    const res = await adminAPI.imageChannelMonitor.list(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = res.items || []
    pagination.total = res.total
    void refreshRuntimeStatuses()
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

async function refreshRuntimeStatus(id: number) {
  try {
    const status = await adminAPI.imageChannelMonitor.getStatus(id)
    runtimeStatuses.value = {
      ...runtimeStatuses.value,
      [id]: status,
    }
  } catch {
    // Runtime status is best-effort; the main list/history APIs still carry persisted results.
  }
}

async function refreshRuntimeStatuses() {
  const ids = monitors.value.map((item) => item.id)
  if (ids.length === 0) return
  await Promise.all(ids.map((id) => refreshRuntimeStatus(id)))
}

function switchPanel(panel: ImageMonitorPanel) {
  activePanel.value = panel
  if (panel === 'manual') {
    void loadManualTargets()
    void loadManualAPIKeys()
  }
}

async function loadManualAPIKeys() {
  manualAPIKeysLoading.value = true
  try {
    manualAPIKeys.value = await adminAPI.usage.searchApiKeys(undefined, '')
    if (
      manualAPIKeyID.value > 0 &&
      !manualAPIKeys.value.some((key) => key.id === manualAPIKeyID.value)
    ) {
      manualAPIKeyID.value = 0
    }
  } catch (err: unknown) {
    manualAPIKeys.value = []
    appStore.showError(
      extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.apiKeyLoadFailed'))
    )
  } finally {
    manualAPIKeysLoading.value = false
  }
}

async function loadManualTargets() {
  manualTargetsLoading.value = true
  try {
    const res = await adminAPI.imageChannelMonitor.list({
      page: 1,
      page_size: 200,
    })
    manualTargets.value = res.items || []
    const available = new Set(manualSelectableTargets.value.map((item) => item.id))
    manualSelectedIds.value = manualSelectedIds.value.filter((id) => available.has(id))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.loadError')))
  } finally {
    manualTargetsLoading.value = false
  }
}

function selectAllManualTargets() {
  if (manualExecutionMode.value !== 'direct_probe') {
    const first = manualFilteredTargets.value[0]
    manualSelectedIds.value = first ? [first.id] : []
    return
  }
  const merged = new Set(manualSelectedIds.value)
  manualFilteredTargets.value.forEach((target) => merged.add(target.id))
  manualSelectedIds.value = Array.from(merged)
}

function clearManualTargets() {
  manualSelectedIds.value = []
}

function setManualExecutionMode(mode: ManualExecutionMode) {
  manualExecutionMode.value = mode
  const selectable = new Set(
    (mode === 'direct_probe'
      ? manualTargets.value
      : manualTargets.value.filter(
          (target) => target.source_type === 'account' && Number(target.account_id) > 0
        )
    ).map((target) => target.id)
  )
  const retained = manualSelectedIds.value.filter((id) => selectable.has(id))
  manualSelectedIds.value = mode === 'direct_probe' ? retained : retained.slice(0, 1)
}

function handleManualTargetToggle(targetID: number) {
  const selected = manualSelectedIds.value.includes(targetID)
  if (manualExecutionMode.value !== 'direct_probe') {
    manualSelectedIds.value = selected ? [] : [targetID]
    return
  }
  manualSelectedIds.value = selected
    ? manualSelectedIds.value.filter((id) => id !== targetID)
    : [...manualSelectedIds.value, targetID]
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 250)
}

function resetForm() {
  Object.assign(form, {
    name: '',
    source_type: 'custom',
    endpoint: 'https://api.openai.com',
    api_key: '',
    account_id: null,
    proxy_id: 0,
    model: 'gpt-image-1',
    prompt: 'Generate a simple health-check image with a clean geometric shape.',
    size_mode: 'omit',
    size: defaultStandardSize,
    custom_size: '',
    quality: 'auto',
    n: 1,
    download_image: true,
    response_format: 'url',
    enabled: true,
    public_visible: false,
    public_name: '',
    interval_seconds: 300,
    timeout_seconds: 300,
  })
}

function openCreateDialog() {
  editing.value = null
  resetForm()
  showDialog.value = true
  loadProxyOptions()
}

function openEditDialog(row: ImageChannelMonitor) {
  editing.value = row
  Object.assign(form, {
    name: row.name,
    source_type: row.source_type,
    endpoint: row.endpoint || 'https://api.openai.com',
    api_key: '',
    account_id: row.account_id,
    proxy_id: row.proxy_id || 0,
    model: row.model,
    prompt: row.prompt,
    quality: row.quality,
    n: row.n,
    download_image: row.download_image,
    response_format: normalizeImageResponseFormat(row.response_format),
    enabled: row.enabled,
    public_visible: row.public_visible,
    public_name: row.public_name,
    interval_seconds: row.interval_seconds,
    timeout_seconds: row.timeout_seconds,
  })
  applySizeModeFromStoredValue(row.size)
  showDialog.value = true
  if (form.source_type === 'account') {
    loadAccountOptions()
  } else {
    loadProxyOptions()
  }
}

function closeDialog() {
  showDialog.value = false
  editing.value = null
}

function handleSourceChange() {
  if (form.source_type === 'account') {
    loadAccountOptions()
  } else {
    loadProxyOptions()
  }
}

async function loadAccountOptions() {
  accountsLoading.value = true
  try {
    const res = await adminAPI.accounts.list(1, 100, {
      platform: 'openai',
      type: 'apikey',
      status: 'active',
    })
    accountOptions.value = res.items || []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.accountLoadError')))
  } finally {
    accountsLoading.value = false
  }
}

async function loadProxyOptions() {
  if (proxyOptions.value.length > 0 || proxiesLoading.value) return
  proxiesLoading.value = true
  try {
    proxyOptions.value = await adminAPI.proxies.getAll()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.proxyLoadError')))
  } finally {
    proxiesLoading.value = false
  }
}

function inferSizeMode(size: string): ImageSizeMode {
  const normalized = size.trim()
  if (!normalized) return 'omit'
  if (normalized === 'auto') return 'auto'
  if (standardSizeOptions.some((option) => option.value === normalized)) return 'preset'
  return 'custom'
}

function applySizeModeFromStoredValue(size: string) {
  const normalized = size.trim()
  const mode = inferSizeMode(normalized)
  form.size_mode = mode
  form.size = mode === 'preset' ? normalized : defaultStandardSize
  form.custom_size = mode === 'custom' ? normalized : ''
}

function resolvedPayloadSize() {
  switch (form.size_mode) {
    case 'auto':
      return 'auto'
    case 'preset':
      return form.size.trim()
    case 'custom':
      return form.custom_size.trim()
    default:
      return ''
  }
}

function resolvedManualSizeFromSettings(settings: ManualPresetSettings) {
  switch (settings.size_mode) {
    case 'auto':
      return 'auto'
    case 'preset':
      return settings.size.trim()
    case 'custom':
      return settings.custom_size.trim()
    default:
      return ''
  }
}

function buildPayload() {
  const payload = {
    name: form.name,
    source_type: form.source_type,
    model: form.model,
    prompt: form.prompt,
    size: resolvedPayloadSize(),
    quality: form.quality,
    n: form.n,
    download_image: form.download_image,
    response_format: form.response_format,
    enabled: form.enabled,
    public_visible: form.public_visible,
    public_name: form.public_name,
    interval_seconds: form.interval_seconds,
    timeout_seconds: form.timeout_seconds,
    endpoint: undefined as string | undefined,
    api_key: undefined as string | undefined,
    account_id: undefined as number | null | undefined,
    proxy_id: undefined as number | null | undefined,
  }
  if (form.source_type === 'custom') {
    payload.endpoint = form.endpoint
    payload.proxy_id = form.proxy_id || 0
    if (!editing.value || form.api_key.trim()) {
      payload.api_key = form.api_key.trim()
    }
    payload.account_id = null
  } else {
    payload.account_id = form.account_id
    payload.proxy_id = 0
  }
  return payload
}

async function handleManualImageChange(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  if (files.length === 0) {
	revokeManualInputObjectURLs()
    manualInputImages.value = []
    return
  }
  try {
	revokeManualInputObjectURLs()
    manualInputImages.value = files.map((file) => ({
		data: createManualInputObjectURL(file),
      blob: file,
      type: file.type || 'image/png',
      name: file.name,
    }))
  } catch {
	revokeManualInputObjectURLs()
    manualInputImages.value = []
    appStore.showError(t('admin.imageChannelMonitor.manual.imageReadError'))
  }
}

function clearManualInputImage() {
	revokeManualInputObjectURLs()
  manualInputImages.value = []
  manualImageInputKey.value += 1
}

function removeManualInputImage(index: number) {
	const removed = manualInputImages.value[index]
	if (removed?.data.startsWith('blob:')) {
		URL.revokeObjectURL(removed.data)
		manualInputObjectURLs.delete(removed.data)
	}
  manualInputImages.value = manualInputImages.value.filter((_, itemIndex) => itemIndex !== index)
  manualImageInputKey.value += 1
}

function createManualInputObjectURL(blob: Blob) {
	const objectURL = URL.createObjectURL(blob)
	manualInputObjectURLs.add(objectURL)
	return objectURL
}

function revokeManualInputObjectURLs() {
	for (const objectURL of manualInputObjectURLs) URL.revokeObjectURL(objectURL)
	manualInputObjectURLs.clear()
}

async function manualInputImageBlob(image?: ManualInputImage | null): Promise<Blob | undefined> {
  if (!image) return undefined
  if (image.blob?.size) return image.blob
  if (!image.data) return undefined
  const response = await fetch(image.data)
  const blob = await response.blob()
  return blob.size ? blob : undefined
}

async function startManualTests() {
  const seq = manualRunSeq + 1
  manualRunSeq = seq
  manualStartRetryAbortController?.abort()
  const startRetryController = new AbortController()
  manualStartRetryAbortController = startRetryController
  const selectableIDs = new Set(manualSelectableTargets.value.map((target) => target.id))
  const ids = manualSelectedIds.value.filter((id) => selectableIDs.has(id))
  if (ids.length === 0) {
    appStore.showError(t('admin.imageChannelMonitor.manual.selectTargetsFirst'))
    return
  }
  if (manualExecutionMode.value !== 'direct_probe' && manualAPIKeyID.value <= 0) {
    appStore.showError(t('admin.imageChannelMonitor.manual.selectApiKeyFirst'))
    return
  }
  if (manualExecutionMode.value !== 'direct_probe' && ids.length !== 1) {
    appStore.showError(t('admin.imageChannelMonitor.manual.gatewaySingleTargetRequired'))
    return
  }
  const targetsById = new Map(manualTargets.value.map((item) => [item.id, item]))
  const selectedTargets = ids
    .map((id) => targetsById.get(id))
    .filter((item): item is ImageChannelMonitor => Boolean(item))
  if (selectedTargets.length === 0) return
  if (
    manualExecutionMode.value === 'gateway_account' &&
    (selectedTargets[0].source_type !== 'account' || Number(selectedTargets[0].account_id) <= 0)
  ) {
    appStore.showError(t('admin.imageChannelMonitor.manual.gatewayAccountTargetRequired'))
    return
  }

  const manualSettings = currentManualPresetSettings()
  const concurrency = clampManualConcurrency(manualSettings.concurrency)
  if (manualSettings.concurrency !== concurrency) {
    manualForm.concurrency = concurrency
  }
  const totalRunCount = selectedTargets.length * concurrency
  if (totalRunCount > manualRunTotalMax) {
    appStore.showError(
      t('admin.imageChannelMonitor.manual.runLimitExceeded', {
        total: totalRunCount,
        max: manualRunTotalMax,
      })
    )
    return
  }
  if (manualSettings.mode === 'edit' && manualInputImages.value.length === 0) {
    appStore.showError(t('admin.imageChannelMonitor.manual.inputPoolRequired'))
    return
  }
  const manualBatchID = newManualBatchID()
  const manualInputImageSnapshots =
    manualSettings.mode === 'edit'
      ? manualInputImages.value.slice(0, totalRunCount).map((image) => ({ ...image }))
      : []
  const basePayload: ImageChannelManualTestParams = {
    mode: manualSettings.mode,
    model: manualSettings.model,
    prompt: manualSettings.prompt,
    size: resolvedManualSizeFromSettings(manualSettings),
    quality: manualSettings.quality,
    n: manualSettings.n,
    download_image: manualSettings.download_image,
    response_format: manualSettings.response_format,
    timeout_seconds: manualSettings.timeout_seconds,
  }
  const runRequests = buildManualRunRequests({
    targetIds: selectedTargets.map((target) => target.id),
    concurrency,
    batchId: manualBatchID,
    basePayload,
    inputImages: manualInputImageSnapshots,
  }).map((request) => {
    const target = targetsById.get(request.targetId) as ImageChannelMonitor
    const expectedAccountID =
      manualExecutionMode.value === 'gateway_account' ? Number(target.account_id) : undefined
    const payload = {
      ...request.payload,
      execution_mode: manualExecutionMode.value,
      ...(manualExecutionMode.value === 'direct_probe'
        ? {}
        : { api_key_id: manualAPIKeyID.value }),
      ...(expectedAccountID ? { expected_account_id: expectedAccountID } : {}),
      client_run_id: newManualClientRunID(),
    } as ManualGatewayTestParams
    return {
      ...request,
      target,
      payload,
      inputImage: request.inputImage || null,
    }
  })
  const manualStartedAt = new Date().toISOString()
  manualRunning.value = true
  manualBatchDismissed.value = false
  manualActiveBatchID.value = manualBatchID
  manualResults.value = Object.fromEntries(
    runRequests.map(({ recordId, target, batchIndex, payload, inputImage }) => [
      recordId,
      {
        recordId,
        monitor: target,
        state: 'running',
        message: t('admin.imageChannelMonitor.manual.requesting'),
        batch_id: manualBatchID,
        batch_size: totalRunCount,
        batch_index: batchIndex,
        clientRunID: payload.client_run_id,
        settings: manualSettings,
        inputImage,
        startedAt: manualStartedAt,
      } satisfies ManualResultItem,
    ])
  )

  try {
    await Promise.allSettled(
      runRequests.map(async ({ recordId, target, batchIndex, payload, inputImage }) => {
        try {
          const run = await startManualRunWithRetry(
            target.id,
            payload,
            await manualInputImageBlob(inputImage),
            seq,
            startRetryController.signal
          )
          if (manualRunSeq !== seq) return
          setManualResultFromRun(recordId, target, run, manualSettings)
          if (run.running) {
            await pollManualRun(
              recordId,
              target,
              run.run_id,
              payload.execution_mode,
              payload.timeout_seconds || manualSettings.timeout_seconds,
              seq
            )
          }
        } catch (err: unknown) {
          if (manualRunSeq !== seq || err instanceof ManualRunCanceledError) return
          const controlObservationLost = isManualRetryableRequestError(err)
          setManualResult(recordId, {
            recordId,
            monitor: target,
            state: controlObservationLost ? 'observation_lost' : 'error',
            message: controlObservationLost
              ? t('admin.imageChannelMonitor.manual.controlObservationLost')
              : extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.failed')),
            observationWarning: controlObservationLost
              ? t('admin.imageChannelMonitor.manual.controlObservationLostDetail')
              : undefined,
            batch_id: manualBatchID,
            batch_size: totalRunCount,
            batch_index: batchIndex,
            settings: manualSettings,
            inputImage,
            startedAt: manualStartedAt,
            completedAt: new Date().toISOString(),
          })
        }
      })
    )
  } finally {
    manualCanceledRunSeqs.delete(seq)
    if (manualStartRetryAbortController === startRetryController) {
      manualStartRetryAbortController = null
    }
    if (manualRunSeq === seq) {
      manualRunning.value = false
    }
  }
}

class ManualRunCanceledError extends Error {}

function newManualClientRunID() {
  return `mcr-${newManualPresetID()}`
}

function isManualObservationExpiredError(error: unknown) {
  if (!error || typeof error !== 'object') return false
  return Number((error as { status?: number }).status) === 410
}

async function startManualRunWithRetry(
  targetID: number,
  payload: ManualGatewayTestParams,
  inputImage: Blob | undefined,
  seq: number,
  signal: AbortSignal
) {
  const idempotent = payload.execution_mode !== 'direct_probe'
  for (let attempt = 0; ; attempt += 1) {
    try {
      const run = await adminAPI.imageChannelMonitor.manualTest(targetID, payload, inputImage)
      if (manualRunSeq !== seq || signal.aborted) {
        if (manualCanceledRunSeqs.has(seq) && run.run_id) {
          try {
            await adminAPI.imageChannelMonitor.cancelManualTest(targetID, run.run_id)
          } catch {
            // The user already canceled locally; a later status fetch can reconcile a failed cancel.
          }
        }
        throw new ManualRunCanceledError()
      }
      return run
    } catch (error: unknown) {
      if (error instanceof ManualRunCanceledError) throw error
      if (!idempotent || !isManualRetryableRequestError(error)) throw error
      await waitForManualStartRetry(
        Math.min(12_000, manualRetryDelayMs(attempt + 1)),
        signal
      )
      if (manualRunSeq !== seq) throw new ManualRunCanceledError()
    }
  }
}

function waitForManualStartRetry(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(new ManualRunCanceledError())
      return
    }
    const timer = window.setTimeout(() => {
      signal.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    const onAbort = () => {
      window.clearTimeout(timer)
      signal.removeEventListener('abort', onAbort)
      reject(new ManualRunCanceledError())
    }
    signal.addEventListener('abort', onAbort, { once: true })
  })
}

async function pollManualRun(
  recordId: string,
  target: ImageChannelMonitor,
  runID: string,
  executionMode: ManualExecutionMode,
  timeoutSeconds: number,
  seq: number
) {
  const maxPolls = manualRunObservationMaxAttempts(executionMode, timeoutSeconds)
  try {
    const terminalRun = await pollManualRunUntilTerminal({
      maxAttempts: maxPolls,
      maxDurationMs: maxPolls === undefined ? undefined : maxPolls * 1000,
      fetchStatus: async () => {
        const run = await adminAPI.imageChannelMonitor.getManualTestStatus(target.id, runID)
        if (manualRunSeq === seq) {
          setManualResultFromRun(recordId, target, run)
        }
        return run
      },
      wait: async () => {
        await wait(1000)
        if (manualRunSeq !== seq) throw new ManualRunCanceledError()
      },
      retryWait: async (attempt) => {
        await wait(Math.min(1000, manualRetryDelayMs(attempt)))
        if (manualRunSeq !== seq) throw new ManualRunCanceledError()
      },
      onObservationError: (_error, attempt) => {
        if (manualRunSeq !== seq) return
        const existing = manualResults.value[recordId]
        if (!existing) return
        setManualResult(recordId, {
          ...existing,
          state: 'running',
          observationWarning: t('admin.imageChannelMonitor.manual.observationWarning', {
            attempt,
          }),
        })
      },
    })
    if (manualRunSeq !== seq) return
    setManualResultFromRun(recordId, target, terminalRun)
  } catch (error: unknown) {
    if (manualRunSeq !== seq || error instanceof ManualRunCanceledError) return
    if (isManualObservationExpiredError(error)) {
      const existing = manualResults.value[recordId]
      if (!existing) return
      setManualResult(recordId, {
        ...existing,
        state: 'observation_lost',
        message: t('admin.imageChannelMonitor.manual.observationLostDetail'),
        observationWarning: t('admin.imageChannelMonitor.manual.observationLostDetail'),
        completedAt: new Date().toISOString(),
      })
      return
    }
    if (!(error instanceof ManualRunObservationError)) throw error
    const existing = manualResults.value[recordId]
    if (!existing) return
    setManualResult(recordId, {
      ...existing,
      state: 'observation_lost',
      message: t('admin.imageChannelMonitor.manual.observationLostDetail'),
      observationWarning: t('admin.imageChannelMonitor.manual.observationLostDetail'),
      completedAt: new Date().toISOString(),
    })
  }
}

async function cancelManualRun(item: ManualResultItem, seq = manualRunSeq) {
  const runID = item.run?.run_id
  if (!runID || item.state !== 'running') return
  try {
    const run = await retryManualCancellation(() =>
      adminAPI.imageChannelMonitor.cancelManualTest(item.monitor.id, runID)
    )
    if (manualRunSeq === seq && manualResults.value[item.recordId]) {
      setManualResultFromRun(item.recordId, item.monitor, run)
    }
  } catch (err: unknown) {
    if (err instanceof ManualRunCanceledError) return
    if (manualRunSeq === seq) {
      appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.cancelFailed')))
    }
  }
}

async function cancelManualRunByClientID(item: ManualResultItem, seq = manualRunSeq) {
  if (!item.clientRunID || item.state !== 'running') return
  try {
    await retryManualCancellation(() =>
      adminAPI.imageChannelMonitor.cancelManualTestByClientRunID(
        item.monitor.id,
        item.clientRunID as string
      )
    )
    if (manualRunSeq === seq && manualResults.value[item.recordId]) {
      markManualResultCanceled(item)
    }
  } catch (err: unknown) {
    if (err instanceof ManualRunCanceledError) return
    if (manualRunSeq === seq) {
      appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.cancelFailed')))
    }
  }
}

async function retryManualCancellation(
  cancel: () => Promise<ImageChannelManualRunResponse>
): Promise<ImageChannelManualRunResponse> {
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await cancel()
    } catch (error: unknown) {
      if (!isManualRetryableRequestError(error)) throw error
      await wait(manualRetryDelayMs(attempt + 1))
      if (manualViewDisposed) throw new ManualRunCanceledError()
    }
  }
}

function markManualResultCanceled(item: ManualResultItem) {
  const existing = manualResults.value[item.recordId]
  if (!existing || existing.state !== 'running') return
  const completedAt = new Date().toISOString()
  if (existing.run?.run_id) {
    setManualResultFromRun(
      item.recordId,
      item.monitor,
      {
        ...existing.run,
        running: false,
        canceled: true,
        stage: 'canceled',
        message: t('admin.imageChannelMonitor.manual.canceled'),
        updated_at: completedAt,
        completed_at: completedAt,
        gateway_status: 'canceled',
        delivery_status: 'canceled',
        observation_status: 'observable',
        artifacts: [],
        result: undefined,
      },
      existing.settings
    )
    return
  }
  setManualResult(item.recordId, {
    ...existing,
    state: 'canceled',
    message: t('admin.imageChannelMonitor.manual.canceled'),
    completedAt,
  })
}

function cancelRunningManualTests() {
  const seq = manualRunSeq
  manualCanceledRunSeqs.add(seq)
  manualStartRetryAbortController?.abort()
  const runningItems = manualResultList.value.filter((item) => item.state === 'running')
  runningItems.forEach(markManualResultCanceled)
  if (manualRunSeq === seq) {
    manualRunning.value = false
  }
  if (runningItems.length === 0) return
  void Promise.allSettled(
    runningItems.map((item) => {
      if (item.run?.run_id) return cancelManualRun(item, seq)
      return cancelManualRunByClientID(item, seq)
    })
  )
}

function setManualResultFromRun(
  recordId: string,
  target: ImageChannelMonitor,
  run: ImageChannelManualRunResponse,
  settings?: ManualPresetSettings
) {
  const existing = manualResults.value[recordId]
  const observationExpired = manualRunStatusField(run, 'observation_status') === 'expired'
  const next: ManualResultItem = {
    recordId,
    monitor: run.monitor || target,
    state: observationExpired
      ? 'observation_lost'
      : run.running
        ? 'running'
        : run.canceled
          ? 'canceled'
          : run.result
            ? 'done'
            : 'error',
    message:
      run.message ||
      run.result?.message ||
      (observationExpired ? t('admin.imageChannelMonitor.manual.observationLostDetail') : ''),
    observationWarning: observationExpired
      ? t('admin.imageChannelMonitor.manual.observationLostDetail')
      : undefined,
    artifactWarning: existing?.artifactWarning,
    batch_id: run.batch_id || existing?.batch_id || '',
    batch_size: run.batch_size || existing?.batch_size || 0,
    batch_index: run.batch_index || existing?.batch_index || 0,
    clientRunID: run.client_run_id || existing?.clientRunID,
    run,
    settings: settings || existing?.settings,
    inputImage: existing?.inputImage,
    startedAt: existing?.startedAt || run.started_at,
    completedAt: run.completed_at || existing?.completedAt,
  }
  setManualResult(recordId, {
    ...next,
  })
  if (!run.running) {
    void finalizeManualRun(recordId, target, run, next)
  }
}

async function finalizeManualRun(
  recordId: string,
  target: ImageChannelMonitor,
  run: ImageChannelManualRunResponse,
  item: ManualResultItem
) {
  if (!run.run_id || manualTerminalPendingRunIDs.has(run.run_id)) return
  if (manualHistory.value.some((history) => history.run_id === run.run_id)) return
  manualTerminalPendingRunIDs.add(run.run_id)
  try {
    const artifactLoad = await loadManualRunArtifact(recordId, target, run)
    await appendManualHistoryFromRun(target, item, artifactLoad)
  } finally {
    manualTerminalPendingRunIDs.delete(run.run_id)
  }
}

function manualRunArtifactIndex(run: ImageChannelManualRunResponse): number | null {
  const artifacts = manualRunExtendedFields(run).artifacts
  if (Array.isArray(artifacts)) {
	const indexes = artifacts
	  .map((artifact) => Number(artifact.index))
	  .filter((index) => Number.isInteger(index) && index >= 0)
	return indexes.length > 0 ? Math.min(...indexes) : null
  }
  const result = run.result
	return (
    result?.returned_image_data ||
      result?.returned_image_url ||
      result?.has_b64_json ||
      result?.has_url
  )
	  ? 0
	  : null
}

async function loadManualRunArtifact(
  recordId: string,
  target: ImageChannelMonitor,
  run: ImageChannelManualRunResponse
): Promise<ManualArtifactLoadResult> {
  const inlineImage = run.result?.returned_image_data
    ? dataURLToManualInputImage(run.result.returned_image_data, 'generated-image')
    : null
	const artifactIndex = manualRunArtifactIndex(run)
  if (!run.run_id || artifactIndex === null) {
    return { image: inlineImage, index: artifactIndex, pending: false }
  }
  if (manualArtifactPendingRunIDs.has(run.run_id) || manualRunOutputPreviews.value[run.run_id]) {
    return { image: inlineImage, index: artifactIndex, pending: false }
  }
  manualArtifactPendingRunIDs.add(run.run_id)
  try {
    const timeoutSeconds =
      manualResults.value[recordId]?.settings?.timeout_seconds || manualForm.timeout_seconds
    const blob = await getManualRunArtifactWithRetry(
      target.id,
      run.run_id,
	  artifactIndex,
      timeoutSeconds
    )
    const storedImage = manualPersistedBlob(blob, `generated-image-${run.run_id}`)
    manualPendingOutputImages.set(run.run_id, storedImage)
    setManualRunOutputPreview(run.run_id, storedImage)
    const existing = manualResults.value[recordId]
    if (existing) {
      setManualResult(recordId, { ...existing, artifactWarning: undefined })
    }
    return { image: storedImage, index: artifactIndex, pending: false }
  } catch (error: unknown) {
    const existing = manualResults.value[recordId]
    if (existing) {
      setManualResult(recordId, {
        ...existing,
        artifactWarning: t('admin.imageChannelMonitor.manual.artifactUnavailable'),
      })
    }
    return {
      image: inlineImage,
      index: artifactIndex,
      pending: !inlineImage && isManualRetryableRequestError(error),
    }
  } finally {
    manualArtifactPendingRunIDs.delete(run.run_id)
  }
}

function setManualRunOutputPreview(runID: string, image: ManualPersistedImage) {
  if (!image.blob?.size || typeof URL.createObjectURL !== 'function') return
  const objectURL = URL.createObjectURL(image.blob)
  const previous = manualOutputObjectURLs.get(runID)
  if (previous) URL.revokeObjectURL(previous)
  manualOutputObjectURLs.set(runID, objectURL)
  manualRunOutputPreviews.value = {
    ...manualRunOutputPreviews.value,
    [runID]: objectURL,
  }
}

async function getManualRunArtifactWithRetry(
  targetID: number,
  runID: string,
	artifactIndex: number,
  timeoutSeconds: number
) {
  const maxRetries = 2
  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    try {
      const blob = await adminAPI.imageChannelMonitor.getManualTestImage(targetID, runID, artifactIndex, {
        timeoutSeconds,
      })
      if (!blob.size) throw { status: 0, code: 'EMPTY_ARTIFACT' }
      return blob
    } catch (error: unknown) {
      if (!isManualRetryableRequestError(error) || attempt === maxRetries) throw error
      await wait(manualRetryDelayMs(attempt + 1))
    }
  }
  throw new Error('unreachable')
}

function manualPersistedBlob(blob: Blob, fallbackName: string): ManualPersistedImage {
  const type = blob.type || 'image/png'
  const ext = type.split('/')[1]?.split(';')[0] || 'png'
  return {
    blob,
    type,
    name: `${fallbackName}.${ext}`,
  }
}

function manualRetryDelayMs(attempt: number) {
  return Math.min(2_400, 300 * 2 ** Math.max(0, attempt - 1))
}

function setManualResult(id: string, item: ManualResultItem) {
  manualResults.value = {
    ...manualResults.value,
    [id]: item,
  }
}

async function saveMonitor() {
  saving.value = true
  try {
    if (editing.value) {
      await adminAPI.imageChannelMonitor.update(editing.value.id, buildPayload())
      appStore.showSuccess(t('admin.imageChannelMonitor.updateSuccess'))
    } else {
      await adminAPI.imageChannelMonitor.create(buildPayload())
      appStore.showSuccess(t('admin.imageChannelMonitor.createSuccess'))
    }
    closeDialog()
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.saveError')))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(row: ImageChannelMonitor) {
  try {
    await adminAPI.imageChannelMonitor.update(row.id, { enabled: !row.enabled })
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.saveError')))
  }
}

async function runNow(row: ImageChannelMonitor) {
  runningId.value = row.id
  lastRunResult.value = null
  try {
    const status = await adminAPI.imageChannelMonitor.runNow(row.id)
    runtimeStatuses.value = {
      ...runtimeStatuses.value,
      [row.id]: status,
    }
    appStore.showSuccess(t('admin.imageChannelMonitor.runSuccess'))
    void pollRunningMonitor(row.id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.runFailed')))
  } finally {
    runningId.value = null
  }
}

async function pollRunningMonitor(id: number) {
  for (let i = 0; i < 180; i += 1) {
    await refreshRuntimeStatus(id)
    const current = runtimeStatuses.value[id]
    if (!current?.running) {
      await reload()
      return
    }
    await wait(1000)
  }
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

async function openHistory(row: ImageChannelMonitor) {
  try {
    historyItems.value = await adminAPI.imageChannelMonitor.listHistory(row.id, { limit: 100 })
    showHistoryDialog.value = true
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.historyLoadError')))
  }
}

function askDelete(row: ImageChannelMonitor) {
  deleting.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.imageChannelMonitor.del(deleting.value.id)
    appStore.showSuccess(t('admin.imageChannelMonitor.deleteSuccess'))
    showDeleteDialog.value = false
    deleting.value = null
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.deleteError')))
  }
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  reload()
}

function sourceLabel(source: ImageMonitorSourceType) {
  return source === 'account'
    ? t('admin.imageChannelMonitor.sourceAccount')
    : t('admin.imageChannelMonitor.sourceCustom')
}

function sourceBadgeClass(source: ImageMonitorSourceType) {
  return source === 'account'
    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
    : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function statusLabel(status: ImageMonitorStatus) {
  return t(`admin.imageChannelMonitor.status.${status}`)
}

function statusBadgeClass(status: ImageMonitorStatus) {
  switch (status) {
    case 'operational':
      return 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-200'
    case 'degraded':
      return 'bg-yellow-50 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-200'
    case 'failed':
      return 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
    default:
      return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  }
}

function runtimeStateLabel(row: ImageChannelMonitor) {
  return runtimeStatuses.value[row.id]?.running
    ? t('admin.imageChannelMonitor.runtime.running')
    : t('admin.imageChannelMonitor.runtime.idle')
}

function runtimeBadgeClass(row: ImageChannelMonitor) {
  return runtimeStatuses.value[row.id]?.running
    ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
    : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
}

function runtimeStageText(row: ImageChannelMonitor) {
  const stage = runtimeStatuses.value[row.id]?.stage || 'idle'
  return t(`admin.imageChannelMonitor.stages.${stage}`, stage)
}

function runtimeMessage(row: ImageChannelMonitor) {
  return runtimeStatuses.value[row.id]?.message || ''
}

function nextCheckText(row: ImageChannelMonitor) {
  if (!row.enabled) return t('admin.imageChannelMonitor.runtime.disabled')
  const status = runtimeStatuses.value[row.id]
  const target = status?.next_check_at || inferNextCheckAt(row)
  if (!target) return t('admin.imageChannelMonitor.runtime.nextCheckUnknown')
  const seconds = Math.max(0, Math.ceil((new Date(target).getTime() - nowMs.value) / 1000))
  return t('admin.imageChannelMonitor.runtime.nextCheckIn', { seconds })
}

function inferNextCheckAt(row: ImageChannelMonitor) {
  if (!row.last_checked_at) return ''
  return new Date(new Date(row.last_checked_at).getTime() + row.interval_seconds * 1000).toISOString()
}

function manualRunResult(item: ManualResultItem) {
  return item.run?.result
}

function manualPreview(item: ManualResultItem) {
  const result = manualRunResult(item)
  const runID = item.run?.run_id || ''
  return (
    manualRunOutputPreviews.value[runID] ||
    result?.returned_image_url ||
    result?.returned_image_data ||
    ''
  )
}

function manualHistoryInputPreview(entry: ManualHistoryItem) {
  return manualHistoryInputPreviews.value[entry.id] || ''
}

function manualHistoryOutputPreview(entry: ManualHistoryItem) {
  return (
    manualRunOutputPreviews.value[entry.run_id] ||
    manualHistoryOutputPreviews.value[entry.id] ||
    entry.result?.returned_image_url ||
    entry.result?.returned_image_data ||
    ''
  )
}

// APIHeaderMs (start→headers), APIBodyMs (headers→body, phase), ImageDownloadMs
// (download, phase) are sequential non-overlapping durations, so they stack cleanly.
function manualWaterfallSegments(result?: ImageChannelMonitorResult): WaterfallSegment[] {
  if (!result) return []
  const raw = [
    { key: 'apiHeader', label: t('admin.imageChannelMonitor.metrics.apiHeader'), ms: result.api_header_ms },
    { key: 'apiBody', label: t('admin.imageChannelMonitor.metrics.apiBody'), ms: result.api_body_ms },
    { key: 'imageDownload', label: t('admin.imageChannelMonitor.metrics.imageDownload'), ms: result.image_download_ms },
  ].filter((seg): seg is { key: string; label: string; ms: number } =>
    typeof seg.ms === 'number' && seg.ms > 0
  )
  const total = raw.reduce((sum, seg) => sum + seg.ms, 0)
  if (total <= 0) return []
  return raw.map((seg) => ({ ...seg, pct: (seg.ms / total) * 100 }))
}

function networkInfoItems(result?: ImageChannelMonitorResult): NetworkInfoItem[] {
  if (!result) return []
  const items: NetworkInfoItem[] = []
  const gatewayResult = result as ImageChannelMonitorResult & {
    gateway_client_request_id?: string
    gateway_request_ids?: string[]
  }
  if (gatewayResult.gateway_client_request_id) {
    items.push({
      label: t('admin.imageChannelMonitor.network.gatewayClientRequestId'),
      value: gatewayResult.gateway_client_request_id,
    })
  }
  if (gatewayResult.gateway_request_ids?.length) {
    items.push({
      label: t('admin.imageChannelMonitor.network.gatewayRequestIds'),
      value: gatewayResult.gateway_request_ids.join(', '),
    })
  }
  if (result.exit_ip) {
    items.push({
      label: t('admin.imageChannelMonitor.network.exitIp'),
      value: result.exit_ip,
    })
  }
  if (result.request_target_url) {
    items.push({
      label: t('admin.imageChannelMonitor.network.requestUrl'),
      value: result.request_target_url,
      href: result.request_target_url,
    })
  }
  if (result.request_target_host) {
    items.push({
      label: t('admin.imageChannelMonitor.network.requestHost'),
      value: result.request_target_host,
    })
  }
  if (result.request_target_ips?.length) {
    items.push({
      label: t('admin.imageChannelMonitor.network.requestIps'),
      value: result.request_target_ips.join(', '),
    })
  }
  if (result.image_download_url || result.returned_image_url) {
    const url = result.image_download_url || result.returned_image_url
    items.push({
      label: t('admin.imageChannelMonitor.network.imageUrl'),
      value: formatReturnedImageURLForDisplay(url),
      href: isDataURL(url) ? undefined : url,
    })
  }
  if (result.image_download_host) {
    items.push({
      label: t('admin.imageChannelMonitor.network.imageHost'),
      value: result.image_download_host,
    })
  }
  if (result.image_download_ips?.length) {
    items.push({
      label: t('admin.imageChannelMonitor.network.imageIps'),
      value: result.image_download_ips.join(', '),
    })
  }
  return items
}

function formatMs(value: number | null) {
  return typeof value === 'number' ? `${value} ms` : '-'
}

// Grouped digits for dense numeric columns / waterfall legend (e.g. 19,482 ms).
function formatMsGrouped(value: number | null | undefined) {
  return typeof value === 'number' ? `${value.toLocaleString()} ms` : '-'
}

function elapsedMs(start: string, end: string) {
  const startMs = new Date(start).getTime()
  const endMs = new Date(end).getTime()
  if (!Number.isFinite(startMs) || !Number.isFinite(endMs)) return 0
  return Math.max(0, endMs - startMs)
}

function formatDuration(ms: number) {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`
  }
  return `${seconds}s`
}

function formatSize(size: string) {
  const normalized = size.trim()
  if (!normalized) return t('admin.imageChannelMonitor.sizeOmitted')
  return normalized
}

function monitorStripPoints(row: ImageChannelMonitor): MonitorTimelinePoint[] {
  return (row.timeline ?? []).map((p) => ({
    status: p.status,
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
}

function rowCountdownSeconds(row: ImageChannelMonitor): number {
  return runtimeStatuses.value[row.id]?.seconds_until_next_check ?? 0
}

// 无任何检查记录时可用率没有意义,显示 '-' 而不是误导性的 0.0%。
function formatAvailability(row: ImageChannelMonitor): string {
  if (!row.timeline?.length || typeof row.availability_7d !== 'number') return '-'
  return `${row.availability_7d.toFixed(1)}%`
}

function manualRecordImageDims(entry: ManualRecordEntry) {
  const width = entry.result?.image_width
  const height = entry.result?.image_height
  if (!width || !height) return ''
  return `${width}x${height}`
}

// Mismatch is only meaningful when the request pinned a concrete WxH size.
function manualRecordSizeMismatch(entry: ManualRecordEntry) {
  const dims = manualRecordImageDims(entry)
  const requested = (entry.size || '').trim().toLowerCase()
  if (!dims || !/^\d+x\d+$/.test(requested)) return false
  return dims !== requested
}

function manualRecordImageBytesText(entry: ManualRecordEntry) {
  return formatImageBytes(entry.result?.image_bytes)
}

function isDataURL(value: string) {
  return /^data:/i.test(value.trim())
}

function dataURLPayloadBytes(value: string) {
  const comma = value.indexOf(',')
  if (comma < 0) return 0
  const payload = value.slice(comma + 1).replace(/\s/g, '')
  if (!payload) return 0
  const padding = payload.endsWith('==') ? 2 : payload.endsWith('=') ? 1 : 0
  return Math.max(0, Math.floor((payload.length * 3) / 4) - padding)
}

function formatReturnedImageURLForDisplay(value: string) {
  const trimmed = value.trim()
  if (!isDataURL(trimmed)) return trimmed
  const mediaType = /^data:([^;,]+)/i.exec(trimmed)?.[1] || 'data'
  const isBase64 = /^data:[^,]*;base64,/i.test(trimmed)
  const bytes = isBase64 ? dataURLPayloadBytes(trimmed) : 0
  const size = bytes > 0 ? ` (${formatBytes(bytes, 1)} inline)` : ''
  return `data:${mediaType}${isBase64 ? ';base64' : ''},...${size}`
}

function formatImageBytes(value: number | null | undefined) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) return ''
  return formatBytes(value, 1)
}

function historyImageInfo(item: ImageChannelMonitorHistoryItem) {
  const dims =
    item.image_width && item.image_height ? `${item.image_width}x${item.image_height}` : ''
  const bytes = formatImageBytes(item.image_bytes)
  if (dims && bytes) return `${dims} · ${bytes}`
  return dims || bytes || '-'
}

function formatDate(value: string | null) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function handleFieldsOutsideClick(event: MouseEvent) {
  const el = fieldsDetails.value
  if (el?.open && !el.contains(event.target as Node)) {
    el.removeAttribute('open')
  }
}

onMounted(() => {
  manualViewDisposed = false
  loadManualPresets()
  loadManualHistory()
  reload()
  document.addEventListener('click', handleFieldsOutsideClick)
  clockTimer = window.setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
  statusPollTimer = window.setInterval(() => {
    void refreshRuntimeStatuses()
  }, 2000)
})

onUnmounted(() => {
  manualViewDisposed = true
  manualRunSeq += 1
  manualStartRetryAbortController?.abort()
  manualStartRetryAbortController = null
  if (abortController) abortController.abort()
  if (searchTimeout) clearTimeout(searchTimeout)
  if (statusPollTimer) clearInterval(statusPollTimer)
  if (clockTimer) clearInterval(clockTimer)
  for (const runID of manualArtifactRecoveryTimers.keys()) {
    cancelManualArtifactRecovery(runID)
  }
  for (const objectURL of manualOutputObjectURLs.values()) {
    URL.revokeObjectURL(objectURL)
  }
  manualOutputObjectURLs.clear()
	manualPendingOutputImages.clear()
	revokeManualInputObjectURLs()
  document.removeEventListener('click', handleFieldsOutsideClick)
})
</script>

<style scoped>
/* Manual-test results table: sticky header + compact cells.
   Rendered inside TablePageLayout's bare-table slot, so it owns all cell styling. */
.mtbl {
  border-collapse: separate;
  border-spacing: 0;
  min-width: 700px;
}

.mtbl-th {
  @apply sticky top-0 z-10 whitespace-nowrap bg-gray-50 px-3 py-2 text-left text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:bg-dark-800 dark:text-dark-400;
  box-shadow: inset 0 -1px 0 rgb(229 231 235);
}

:global(.dark) .mtbl-th {
  box-shadow: inset 0 -1px 0 rgb(51 65 85);
}

.mtbl-th-num {
  @apply text-right;
}

.mtbl-td {
  @apply border-b border-gray-100 px-3 py-2 align-middle text-gray-700 dark:border-dark-800 dark:text-dark-200;
}

.mtbl-td-num {
  @apply whitespace-nowrap text-right font-mono tabular-nums;
}
</style>
