<template>
  <AppLayout>
    <TablePageLayout scroll-mode="fixed" :bare-table="activePanel === 'manual'">
      <template #filters>
        <div class="space-y-4">
          <!-- Panel switcher: compact header + segmented tabs (A) -->
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.imageChannelMonitor.title') }}
              </h1>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ activePanel === 'manual'
                  ? t('admin.imageChannelMonitor.panels.manualDesc')
                  : t('admin.imageChannelMonitor.panels.monitorsDesc') }}
              </p>
            </div>
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
            <section class="flex-none border-b border-gray-200 dark:border-dark-700">
              <div class="flex items-center gap-2 px-4 py-2.5">
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
              <div v-show="!manualConfigCollapsed" class="space-y-3 px-4 pb-4">
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

                <div class="grid grid-cols-3 gap-2.5">
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
                </div>

                <label class="block">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.form.prompt') }}</span>
                  <textarea v-model.trim="manualForm.prompt" rows="2" class="input min-h-[52px]" />
                </label>

                <div v-if="manualForm.mode === 'edit'" class="space-y-2">
                  <span class="input-label">{{ t('admin.imageChannelMonitor.manual.inputImage') }}</span>
                  <input class="input" type="file" accept="image/*" @change="handleManualImageChange" />
                  <div
                    v-if="manualInputImage"
                    class="flex items-center gap-2 rounded-md border border-gray-200 p-1.5 text-xs dark:border-dark-700"
                  >
                    <img :src="manualInputImage.data" class="h-9 w-9 rounded object-cover" alt="" />
                    <span class="min-w-0 flex-1 truncate text-gray-600 dark:text-dark-300">{{ manualInputImage.name }}</span>
                    <button type="button" class="btn btn-secondary btn-sm" @click="clearManualInputImage">
                      {{ t('common.clear') }}
                    </button>
                  </div>
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
                  {{ t('admin.imageChannelMonitor.manual.selectedOfTotal', { selected: manualSelectedIds.length, total: manualTargets.length }) }}
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
                    v-model="manualSelectedIds"
                    type="checkbox"
                    class="h-4 w-4 flex-none rounded border-gray-300 text-primary-600"
                    :value="target.id"
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
                  :disabled="manualSelectedIds.length === 0"
                  @click="startManualTests"
                >
                  {{ t('admin.imageChannelMonitor.manual.startWithCount', { count: manualSelectedIds.length }) }}
                </button>
                <p class="mt-2 text-center text-[11px] text-gray-400 dark:text-dark-500">
                  {{ t('admin.imageChannelMonitor.manual.ctaHint') }}
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
              <td class="py-3 pr-4">{{ formatMs(item.api_total_ms) }}</td>
              <td class="py-3 pr-4">{{ formatMs(item.image_download_ms) }}</td>
              <td class="py-3 pr-4">
                {{ item.image_width && item.image_height ? `${item.image_width}x${item.image_height}` : '-' }}
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
          <MetricItem :label="t('admin.imageChannelMonitor.form.model')" :value="selectedManualRecord.model || '-'" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.size')" :value="formatSize(selectedManualRecord.size)" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.quality')" :value="selectedManualRecord.quality || '-'" />
          <MetricItem :label="'n'" :value="String(selectedManualRecord.n)" />
          <MetricItem :label="t('admin.imageChannelMonitor.form.downloadImage')" :value="selectedManualRecord.download_image ? t('common.yes') : t('common.no')" />
        </dl>

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
import type { Account, Proxy } from '@/types'
import type { Column } from '@/components/common/types'
import type {
  ImageChannelMonitor,
  ImageChannelMonitorHistoryItem,
  ImageChannelMonitorListParams,
  ImageChannelManualRunResponse,
  ImageChannelMonitorResult,
  ImageChannelMonitorRuntimeStatus,
  ImageMonitorSourceType,
  ImageMonitorStatus,
} from '@/api/admin/imageChannelMonitor'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const MetricItem = (_props: { label: string; value: string }) =>
  h('div', { class: 'rounded-md bg-gray-50 p-3 dark:bg-dark-800' }, [
    h('dt', { class: 'text-xs text-gray-500 dark:text-dark-400' }, _props.label),
    h('dd', { class: 'mt-1 font-medium text-gray-900 dark:text-white' }, _props.value),
  ])

const { t } = useI18n()
const appStore = useAppStore()

type ImageSizeMode = 'omit' | 'auto' | 'preset' | 'custom'
type ImageMonitorPanel = 'monitors' | 'manual'

type ManualResultItem = {
  monitor: ImageChannelMonitor
  state: 'running' | 'done' | 'error' | 'canceled'
  message: string
  run?: ImageChannelManualRunResponse
  settings?: ManualPresetSettings
  inputImage?: ManualInputImage | null
  startedAt?: string
  completedAt?: string
}

type ManualPresetSettings = {
  mode: 'generate' | 'edit'
  model: string
  prompt: string
  size_mode: ImageSizeMode
  size: string
  custom_size: string
  quality: string
  n: number
  download_image: boolean
  timeout_seconds: number
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
  mode: 'generate' | 'edit'
  status: ImageMonitorStatus | 'canceled'
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
  input_image_ref?: string
  input_image_type?: string
  input_image_name?: string
  output_image_ref?: string
  output_image_url?: string
  result?: ImageChannelMonitorResult
}

type ManualInputImage = {
  data: string
  type: string
  name: string
}

type ManualStoredImage = ManualInputImage & {
  ref: string
  saved_at: string
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

type ManualRecordStatus = ImageMonitorStatus | 'running' | 'canceled'
type ManualRecordSource = 'live' | 'history'

type ManualRecordColumnKey =
  | 'started_at'
  | 'monitor'
  | 'status'
  | 'mode'
  | 'model'
  | 'size'
  | 'elapsed'
  | 'api_total'
  | 'image_download'
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
  mode: 'generate' | 'edit'
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
const manualResults = ref<Record<number, ManualResultItem>>({})
const manualHistory = ref<ManualHistoryItem[]>([])
const manualHistoryInputPreviews = ref<Record<string, string>>({})
const manualHistoryOutputPreviews = ref<Record<string, string>>({})
const manualPresets = ref<ManualPreset[]>([])
const manualPresetSelectedId = ref('')
const manualPresetName = ref('')
const manualInputImage = ref<ManualInputImage | null>(null)
const manualConfigCollapsed = ref(false)
const manualTargetSearch = ref('')
const manualBatchDismissed = ref(false)
const showManualPresetSaveDialog = ref(false)
const fieldsDetails = ref<HTMLDetailsElement | null>(null)
const selectedManualRecord = ref<ManualRecordEntry | null>(null)
const manualRecordSearch = ref('')
const manualRecordStatusFilter = ref<ManualRecordStatus | ''>('')
const manualRecordModeFilter = ref<'' | 'generate' | 'edit'>('')
const manualRecordMonitorFilter = ref<number | ''>('')
const manualRecordSort = ref<'newest' | 'oldest'>('newest')
const manualVisibleColumns = ref<ManualRecordColumnKey[]>([
  'started_at',
  'monitor',
  'status',
  'elapsed',
  'api_total',
  'image_download',
  'exit_ip',
  'output',
])

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let statusPollTimer: number | null = null
let clockTimer: number | null = null
let manualRunSeq = 0
const manualHistoryPendingRunIDs = new Set<string>()

const manualPresetStorageKey = 'sub2api:image-channel-monitor:manual-presets:v1'
const manualHistoryStorageKey = 'sub2api:image-channel-monitor:manual-history:v1'
const manualImageDBName = 'sub2api-image-channel-monitor'
const manualImageStoreName = 'manual-images'
const defaultStandardSize = '1024x1024'

const standardSizeOptions = [
  { labelKey: 'admin.imageChannelMonitor.sizes.square', value: '1024x1024' },
  { labelKey: 'admin.imageChannelMonitor.sizes.landscape', value: '1536x1024' },
  { labelKey: 'admin.imageChannelMonitor.sizes.portrait', value: '1024x1536' },
]

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
  enabled: true,
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
  timeout_seconds: 300,
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
  Object.values(manualResults.value).sort((a, b) => a.monitor.id - b.monitor.id)
)

const manualFilteredTargets = computed(() => {
  const query = manualTargetSearch.value.trim().toLowerCase()
  if (!query) return manualTargets.value
  return manualTargets.value.filter((target) => {
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

const manualConfigSummary = computed(() => [
  manualForm.mode === 'edit'
    ? t('admin.imageChannelMonitor.manual.edit')
    : t('admin.imageChannelMonitor.manual.generate'),
  manualForm.model || '-',
  manualResolvedSizeLabel.value,
  `n=${manualForm.n}`,
])

const manualBatchStats = computed(() => {
  let running = 0
  let ok = 0
  let failed = 0
  let canceled = 0
  for (const item of manualResultList.value) {
    if (item.state === 'running') {
      running += 1
      continue
    }
    if (item.state === 'canceled') {
      canceled += 1
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
  return { total, running, done: total - running, ok, failed, canceled }
})

const manualShowBatchBanner = computed(
  () => !manualBatchDismissed.value && manualBatchStats.value.total > 0
)

const manualBatchProgress = computed(() => {
  const { total, done } = manualBatchStats.value
  return total > 0 ? Math.round((done / total) * 100) : 0
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
  { key: 'mode', label: t('admin.imageChannelMonitor.manual.columns.mode') },
  { key: 'model', label: t('admin.imageChannelMonitor.manual.columns.model') },
  { key: 'size', label: t('admin.imageChannelMonitor.manual.columns.size') },
  { key: 'elapsed', label: t('admin.imageChannelMonitor.manual.columns.elapsed') },
  { key: 'api_total', label: t('admin.imageChannelMonitor.manual.columns.apiTotal') },
  { key: 'image_download', label: t('admin.imageChannelMonitor.manual.columns.imageDownload') },
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
  return {
    id: item.run?.run_id ? `live-${item.run.run_id}` : `live-${item.monitor.id}`,
    run_id: item.run?.run_id || '',
    source: 'live',
    monitor_id: item.monitor.id,
    monitor_name: item.monitor.name,
    mode: settings.mode,
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
    mode: entry.mode,
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
  if (item.state === 'error') return 'error'
  return manualRunResult(item)?.status || 'error'
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

function manualRecordDownloadName(entry: ManualRecordEntry | null) {
  if (!entry) return 'manual-image-test.png'
  const stamp = entry.started_at ? entry.started_at.replace(/[:.]/g, '-') : 'image'
  return `${sanitizeFileName(entry.monitor_name)}-${stamp}.png`
}

function sanitizeFileName(value: string) {
  return value.trim().replace(/[\\/:*?"<>|]+/g, '-').replace(/\s+/g, '-').slice(0, 80) || 'manual-image-test'
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
    timeout_seconds: clampInt(raw?.timeout_seconds, 300, 30, 600),
    input_image_ref: typeof raw?.input_image_ref === 'string' ? raw.input_image_ref : '',
    input_image_type: typeof raw?.input_image_type === 'string' ? raw.input_image_type : '',
    input_image_name: typeof raw?.input_image_name === 'string' ? raw.input_image_name : '',
  }
}

function normalizeImageSizeMode(value: unknown): ImageSizeMode {
  if (value === 'auto' || value === 'preset' || value === 'custom') {
    return value
  }
  return 'omit'
}

function clampInt(value: unknown, fallback: number, min: number, max: number) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  return Math.min(max, Math.max(min, Math.trunc(parsed)))
}

function currentManualPresetSettings(): ManualPresetSettings {
  return normalizeManualPresetSettings({
    mode: manualForm.mode,
    model: manualForm.model,
    prompt: manualForm.prompt,
    size_mode: manualForm.size_mode,
    size: manualForm.size,
    custom_size: manualForm.custom_size,
    quality: manualForm.quality,
    n: manualForm.n,
    download_image: manualForm.download_image,
    timeout_seconds: manualForm.timeout_seconds,
  })
}

function applyManualPresetSettings(settings: ManualPresetSettings) {
  const normalized = normalizeManualPresetSettings(settings)
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
    timeout_seconds: normalized.timeout_seconds,
  })
}

async function handleManualPresetSelect() {
  const preset = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
  if (!preset) {
    manualPresetName.value = ''
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
      manualInputImage.value = stored
        ? { data: stored.data, type: stored.type, name: stored.name }
        : null
    } catch {
      if (manualPresetSelectedId.value === selectedID) {
        manualInputImage.value = null
      }
    }
  } else {
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
    if (manualForm.mode === 'edit' && manualInputImage.value?.data) {
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
  image: ManualInputImage,
  scope: 'preset-input' | 'history-input' | 'history-output'
) {
  const db = await openManualImageDB()
  const ref = `${scope}:${newManualPresetID()}`
  const stored: ManualStoredImage = {
    ref,
    data: image.data,
    type: image.type,
    name: image.name,
    saved_at: new Date().toISOString(),
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
  image: ManualInputImage | null | undefined,
  scope: 'preset-input' | 'history-input' | 'history-output'
) {
  if (!image?.data) return ''
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
    mode: source.mode === 'edit' ? 'edit' : 'generate',
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
    input_image_ref: typeof source.input_image_ref === 'string' ? source.input_image_ref : '',
    input_image_type: typeof source.input_image_type === 'string' ? source.input_image_type : '',
    input_image_name: typeof source.input_image_name === 'string' ? source.input_image_name : '',
    output_image_ref: typeof source.output_image_ref === 'string' ? source.output_image_ref : '',
    output_image_url: typeof source.output_image_url === 'string' ? source.output_image_url : '',
    result: compactManualHistoryResult(source.result),
  }
}

function normalizeManualHistoryStatus(value: unknown): ImageMonitorStatus | 'canceled' {
  if (
    value === 'operational' ||
    value === 'degraded' ||
    value === 'failed' ||
    value === 'error' ||
    value === 'canceled'
  ) {
    return value
  }
  return 'error'
}

async function appendManualHistoryFromRun(target: ImageChannelMonitor, item: ManualResultItem) {
  const run = item.run
  if (!run?.run_id) return
  if (manualHistory.value.some((entry) => entry.run_id === run.run_id)) return
  if (manualHistoryPendingRunIDs.has(run.run_id)) return
  manualHistoryPendingRunIDs.add(run.run_id)
  try {
    const settings = item.settings || currentManualPresetSettings()
    const completedAt = run.completed_at || new Date().toISOString()
    const result = compactManualHistoryResult(run.result)
    const outputImage = run.result?.returned_image_data
      ? dataURLToManualInputImage(run.result.returned_image_data, 'generated-image')
      : null
    const inputImageRef = await trySaveManualStoredImage(item.inputImage, 'history-input')
    const outputImageRef = await trySaveManualStoredImage(outputImage, 'history-output')
    const entry: ManualHistoryItem = {
      id: run.run_id,
      run_id: run.run_id,
      monitor_id: run.monitor?.id || target.id,
      monitor_name: run.monitor?.name || target.name,
      mode: run.mode,
      status: run.canceled ? 'canceled' : result?.status || 'error',
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
      input_image_ref: inputImageRef,
      input_image_type: item.inputImage?.type || '',
      input_image_name: item.inputImage?.name || '',
      output_image_ref: outputImageRef,
      output_image_url: run.result?.returned_image_url || '',
      result,
    }
    const nextHistory = [
      entry,
      ...manualHistory.value.filter((history) => history.run_id !== run.run_id),
    ]
    const droppedHistory = nextHistory.slice(50)
    manualHistory.value = nextHistory.slice(0, 50)
    persistManualHistory()
    void Promise.allSettled(
      droppedHistory.flatMap((history) => [
        deleteManualStoredImage(history.input_image_ref),
        deleteManualStoredImage(history.output_image_ref),
      ])
    )
    await hydrateManualHistoryImages()
  } finally {
    manualHistoryPendingRunIDs.delete(run.run_id)
  }
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
      if (input?.data) {
        inputPreviews[entry.id] = input.data
      }
      if (output?.data) {
        outputPreviews[entry.id] = output.data
      }
    })
  )
  manualHistoryInputPreviews.value = inputPreviews
  manualHistoryOutputPreviews.value = outputPreviews
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
  manualHistoryInputPreviews.value = {}
  manualHistoryOutputPreviews.value = {}
  if (selectedManualRecord.value?.source === 'history') {
    selectedManualRecord.value = null
  }
  persistManualHistory()
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
    const available = new Set(manualTargets.value.map((item) => item.id))
    manualSelectedIds.value = manualSelectedIds.value.filter((id) => available.has(id))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.loadError')))
  } finally {
    manualTargetsLoading.value = false
  }
}

function selectAllManualTargets() {
  const merged = new Set(manualSelectedIds.value)
  manualFilteredTargets.value.forEach((target) => merged.add(target.id))
  manualSelectedIds.value = Array.from(merged)
}

function clearManualTargets() {
  manualSelectedIds.value = []
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
    enabled: true,
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
    enabled: row.enabled,
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
    enabled: form.enabled,
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
  const file = input.files?.[0]
  if (!file) {
    manualInputImage.value = null
    return
  }
  try {
    manualInputImage.value = {
      data: await readFileAsDataURL(file),
      type: file.type || 'image/png',
      name: file.name,
    }
  } catch {
    manualInputImage.value = null
    appStore.showError(t('admin.imageChannelMonitor.manual.imageReadError'))
  }
}

function clearManualInputImage() {
  manualInputImage.value = null
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

async function startManualTests() {
  const seq = manualRunSeq + 1
  manualRunSeq = seq
  const ids = [...manualSelectedIds.value]
  if (ids.length === 0) {
    appStore.showError(t('admin.imageChannelMonitor.manual.selectTargetsFirst'))
    return
  }
  if (manualForm.mode === 'edit' && !manualInputImage.value?.data) {
    appStore.showError(t('admin.imageChannelMonitor.manual.selectImageFirst'))
    return
  }
  const targetsById = new Map(manualTargets.value.map((item) => [item.id, item]))
  const selectedTargets = ids
    .map((id) => targetsById.get(id))
    .filter((item): item is ImageChannelMonitor => Boolean(item))
  if (selectedTargets.length === 0) return

  const manualSettings = currentManualPresetSettings()
  const manualInputImageSnapshot =
    manualSettings.mode === 'edit' && manualInputImage.value
      ? { ...manualInputImage.value }
      : null
  const manualStartedAt = new Date().toISOString()
  manualRunning.value = true
  manualBatchDismissed.value = false
  manualResults.value = Object.fromEntries(
    selectedTargets.map((target) => [
      target.id,
      {
        monitor: target,
        state: 'running',
        message: t('admin.imageChannelMonitor.manual.requesting'),
        settings: manualSettings,
        inputImage: manualInputImageSnapshot,
        startedAt: manualStartedAt,
      } satisfies ManualResultItem,
    ])
  )

  const payload = {
    mode: manualSettings.mode,
    model: manualSettings.model,
    prompt: manualSettings.prompt,
    size: resolvedManualSizeFromSettings(manualSettings),
    quality: manualSettings.quality,
    n: manualSettings.n,
    download_image: manualSettings.download_image,
    timeout_seconds: manualSettings.timeout_seconds,
    input_image_data: manualSettings.mode === 'edit' ? manualInputImageSnapshot?.data : undefined,
    input_image_type: manualSettings.mode === 'edit' ? manualInputImageSnapshot?.type : undefined,
    input_image_name: manualSettings.mode === 'edit' ? manualInputImageSnapshot?.name : undefined,
  }

  try {
    await Promise.allSettled(
      selectedTargets.map(async (target) => {
        try {
          const run = await adminAPI.imageChannelMonitor.manualTest(target.id, payload)
          if (manualRunSeq !== seq) return
          setManualResultFromRun(target, run, manualSettings)
          if (run.running) {
            await pollManualRun(target, run.run_id, payload.timeout_seconds, seq)
          }
        } catch (err: unknown) {
          if (manualRunSeq !== seq) return
          setManualResult(target.id, {
            monitor: target,
            state: 'error',
            message: extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.failed')),
            settings: manualSettings,
            inputImage: manualInputImageSnapshot,
            startedAt: manualStartedAt,
            completedAt: new Date().toISOString(),
          })
        }
      })
    )
  } finally {
    if (manualRunSeq === seq) {
      manualRunning.value = false
    }
  }
}

async function pollManualRun(
  target: ImageChannelMonitor,
  runID: string,
  timeoutSeconds: number,
  seq: number
) {
  const maxPolls = Math.min(720, Math.max(30, timeoutSeconds + 45))
  for (let i = 0; i < maxPolls; i += 1) {
    await wait(1000)
    if (manualRunSeq !== seq) return
    try {
      const run = await adminAPI.imageChannelMonitor.getManualTestStatus(target.id, runID)
      if (manualRunSeq !== seq) return
      setManualResultFromRun(target, run)
      if (!run.running) return
    } catch (err: unknown) {
      if (manualRunSeq !== seq) return
      setManualResult(target.id, {
        monitor: target,
        state: 'error',
        message: extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.failed')),
        settings: manualResults.value[target.id]?.settings,
        inputImage: manualResults.value[target.id]?.inputImage,
        startedAt: manualResults.value[target.id]?.startedAt,
        completedAt: new Date().toISOString(),
      })
      return
    }
  }
  if (manualRunSeq !== seq) return
  setManualResult(target.id, {
    monitor: target,
    state: 'error',
    message: t('admin.imageChannelMonitor.manual.failed'),
    settings: manualResults.value[target.id]?.settings,
    inputImage: manualResults.value[target.id]?.inputImage,
    startedAt: manualResults.value[target.id]?.startedAt,
    completedAt: new Date().toISOString(),
  })
}

async function cancelManualRun(item: ManualResultItem) {
  const runID = item.run?.run_id
  if (!runID || item.state !== 'running') return
  try {
    const run = await adminAPI.imageChannelMonitor.cancelManualTest(item.monitor.id, runID)
    setManualResultFromRun(item.monitor, run)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.cancelFailed')))
  }
}

async function cancelRunningManualTests() {
  const runningItems = manualResultList.value.filter((item) => item.state === 'running')
  if (runningItems.length === 0) return
  await Promise.allSettled(runningItems.map((item) => cancelManualRun(item)))
}

function setManualResultFromRun(
  target: ImageChannelMonitor,
  run: ImageChannelManualRunResponse,
  settings?: ManualPresetSettings
) {
  const existing = manualResults.value[target.id]
  const next: ManualResultItem = {
    monitor: run.monitor || target,
    state: run.running ? 'running' : run.canceled ? 'canceled' : run.result ? 'done' : 'error',
    message: run.message || run.result?.message || '',
    run,
    settings: settings || existing?.settings,
    inputImage: existing?.inputImage,
    startedAt: existing?.startedAt || run.started_at,
    completedAt: run.completed_at || existing?.completedAt,
  }
  setManualResult(target.id, {
    ...next,
  })
  if (!run.running) {
    void appendManualHistoryFromRun(target, next)
  }
}

function setManualResult(id: number, item: ManualResultItem) {
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
  return result?.returned_image_url || result?.returned_image_data || ''
}

function manualHistoryInputPreview(entry: ManualHistoryItem) {
  return manualHistoryInputPreviews.value[entry.id] || ''
}

function manualHistoryOutputPreview(entry: ManualHistoryItem) {
  return (
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
      value: url,
      href: url,
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
  manualRunSeq += 1
  if (abortController) abortController.abort()
  if (searchTimeout) clearTimeout(searchTimeout)
  if (statusPollTimer) clearInterval(statusPollTimer)
  if (clockTimer) clearInterval(clockTimer)
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
