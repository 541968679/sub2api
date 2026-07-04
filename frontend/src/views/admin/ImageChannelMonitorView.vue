<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <div class="grid gap-3 sm:grid-cols-2">
            <button
              type="button"
              class="rounded-lg border p-4 text-left transition"
              :class="activePanel === 'monitors'
                ? 'border-primary-500 bg-primary-50 text-primary-900 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-100'
                : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200'"
              @click="activePanel = 'monitors'"
            >
              <div class="text-sm font-semibold">{{ t('admin.imageChannelMonitor.panels.monitors') }}</div>
              <div class="mt-1 text-xs opacity-80">{{ t('admin.imageChannelMonitor.panels.monitorsDesc') }}</div>
            </button>
            <button
              type="button"
              class="rounded-lg border p-4 text-left transition"
              :class="activePanel === 'manual'
                ? 'border-primary-500 bg-primary-50 text-primary-900 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-100'
                : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200'"
              @click="switchPanel('manual')"
            >
              <div class="text-sm font-semibold">{{ t('admin.imageChannelMonitor.panels.manual') }}</div>
              <div class="mt-1 text-xs opacity-80">{{ t('admin.imageChannelMonitor.panels.manualDesc') }}</div>
            </button>
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
        <div v-if="activePanel === 'manual'" class="space-y-4">
          <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(180px,260px)_auto] lg:items-end">
              <label class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.manual.preset') }}</span>
                <select v-model="manualPresetSelectedId" class="input" @change="handleManualPresetSelect">
                  <option value="">{{ t('admin.imageChannelMonitor.manual.selectPreset') }}</option>
                  <option v-for="preset in manualPresets" :key="preset.id" :value="preset.id">
                    {{ preset.name }}
                  </option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.manual.presetName') }}</span>
                <input
                  v-model.trim="manualPresetName"
                  class="input"
                  :placeholder="t('admin.imageChannelMonitor.manual.presetNamePlaceholder')"
                />
              </label>
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn btn-secondary btn-sm" @click="saveManualPreset">
                  {{ manualPresetSelectedId ? t('admin.imageChannelMonitor.manual.updatePreset') : t('admin.imageChannelMonitor.manual.savePreset') }}
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
            <div class="grid gap-4 md:grid-cols-4">
              <label class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.manual.mode') }}</span>
                <select v-model="manualForm.mode" class="input">
                  <option value="generate">{{ t('admin.imageChannelMonitor.manual.generate') }}</option>
                  <option value="edit">{{ t('admin.imageChannelMonitor.manual.edit') }}</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.form.model') }}</span>
                <input v-model.trim="manualForm.model" class="input" placeholder="gpt-image-1" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.form.sizeMode') }}</span>
                <select v-model="manualForm.size_mode" class="input">
                  <option value="omit">{{ t('admin.imageChannelMonitor.form.sizeModeOmit') }}</option>
                  <option value="auto">{{ t('admin.imageChannelMonitor.form.sizeModeAuto') }}</option>
                  <option value="preset">{{ t('admin.imageChannelMonitor.form.sizeModePreset') }}</option>
                  <option value="custom">{{ t('admin.imageChannelMonitor.form.sizeModeCustom') }}</option>
                </select>
              </label>
              <label v-if="manualForm.size_mode === 'preset'" class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.form.size') }}</span>
                <select v-model="manualForm.size" class="input">
                  <option v-for="option in standardSizeOptions" :key="option.value" :value="option.value">
                    {{ t(option.labelKey) }}
                  </option>
                </select>
              </label>
              <label v-else-if="manualForm.size_mode === 'custom'" class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.form.customSize') }}</span>
                <input
                  v-model.trim="manualForm.custom_size"
                  class="input"
                  :placeholder="t('admin.imageChannelMonitor.form.customSizePlaceholder')"
                />
              </label>
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
            <label class="mt-4 block">
              <span class="input-label">{{ t('admin.imageChannelMonitor.form.prompt') }}</span>
              <textarea v-model.trim="manualForm.prompt" class="input min-h-[96px]" />
            </label>
            <div class="mt-4 grid gap-4 md:grid-cols-[minmax(0,1fr)_220px] md:items-end">
              <label v-if="manualForm.mode === 'edit'" class="block">
                <span class="input-label">{{ t('admin.imageChannelMonitor.manual.inputImage') }}</span>
                <input class="input" type="file" accept="image/*" @change="handleManualImageChange" />
              </label>
              <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                <input
                  v-model="manualForm.download_image"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600"
                />
                {{ t('admin.imageChannelMonitor.form.downloadImage') }}
              </label>
            </div>
          </section>

          <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.imageChannelMonitor.manual.targets') }}
              </h2>
              <div class="flex items-center gap-2">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="manualTargetsLoading" @click="loadManualTargets">
                  {{ manualTargetsLoading ? t('common.loading') : t('common.refresh') }}
                </button>
                <button type="button" class="btn btn-primary btn-sm" :disabled="manualRunning" @click="startManualTests">
                  {{ manualRunning ? t('common.loading') : t('admin.imageChannelMonitor.manual.start') }}
                </button>
              </div>
            </div>
            <div class="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              <label
                v-for="target in manualTargets"
                :key="target.id"
                class="flex items-start gap-3 rounded-md border border-gray-200 p-3 text-sm dark:border-dark-700"
              >
                <input
                  v-model="manualSelectedIds"
                  type="checkbox"
                  class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600"
                  :value="target.id"
                />
                <span class="min-w-0">
                  <span class="block truncate font-medium text-gray-900 dark:text-white">{{ target.name }}</span>
                  <span class="block truncate text-xs text-gray-500 dark:text-dark-400">
                    {{ sourceLabel(target.source_type) }} · {{ target.model }} · {{ formatSize(target.size) }}
                  </span>
                </span>
              </label>
            </div>
          </section>

          <section v-if="manualResultList.length > 0" class="space-y-3">
            <article
              v-for="item in manualResultList"
              :key="item.monitor.id"
              class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="min-w-0">
                  <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                    {{ item.monitor.name }}
                  </h3>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ manualResultStatusText(item) }}
                  </p>
                </div>
                <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="manualResultBadgeClass(item)">
                  {{ manualResultBadgeText(item) }}
                </span>
              </div>
              <div v-if="manualRunResult(item)" class="mt-3 grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
                <div>
                  <dl class="grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
                    <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiHeader')" :value="formatMs(manualRunResult(item)?.api_header_ms ?? null)" />
                    <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiBody')" :value="formatMs(manualRunResult(item)?.api_body_ms ?? null)" />
                    <MetricItem :label="t('admin.imageChannelMonitor.metrics.apiTotal')" :value="formatMs(manualRunResult(item)?.api_total_ms ?? null)" />
                    <MetricItem :label="t('admin.imageChannelMonitor.metrics.imageDownload')" :value="formatMs(manualRunResult(item)?.image_download_ms ?? null)" />
                  </dl>
                  <p v-if="manualRunResult(item)?.message" class="mt-3 text-sm text-red-600 dark:text-red-300">
                    {{ manualRunResult(item)?.error_stage ? `${manualRunResult(item)?.error_stage}: ` : '' }}{{ manualRunResult(item)?.message }}
                  </p>
                  <div v-if="manualRunResult(item)?.stages?.length" class="mt-3 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-dark-400">
                    <span
                      v-for="stage in manualRunResult(item)?.stages"
                      :key="`${item.monitor.id}-${stage.stage}-${stage.at}`"
                      class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800"
                    >
                      {{ t(`admin.imageChannelMonitor.stages.${stage.stage}`, stage.stage) }}
                    </span>
                  </div>
                </div>
                <div
                  v-if="manualPreview(item)"
                  class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
                >
                  <img :src="manualPreview(item)" class="aspect-square w-full object-contain" alt="" />
                </div>
              </div>
            </article>
          </section>
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
  state: 'running' | 'done' | 'error'
  message: string
  run?: ImageChannelManualRunResponse
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
}

type ManualPreset = {
  id: string
  name: string
  settings: ManualPresetSettings
  created_at: string
  updated_at: string
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
const manualPresets = ref<ManualPreset[]>([])
const manualPresetSelectedId = ref('')
const manualPresetName = ref('')
const manualInputImage = ref<{
  data: string
  type: string
  name: string
} | null>(null)

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let statusPollTimer: number | null = null
let clockTimer: number | null = null
let manualRunSeq = 0

const manualPresetStorageKey = 'sub2api:image-channel-monitor:manual-presets:v1'
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
  Object.assign(manualForm, normalizeManualPresetSettings(settings))
}

function handleManualPresetSelect() {
  const preset = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
  if (!preset) {
    manualPresetName.value = ''
    return
  }
  manualPresetName.value = preset.name
  applyManualPresetSettings(preset.settings)
}

function saveManualPreset() {
  const name = manualPresetName.value.trim()
  if (!name) {
    appStore.showError(t('admin.imageChannelMonitor.manual.presetNameRequired'))
    return
  }
  const now = new Date().toISOString()
  const existing = manualPresets.value.find((item) => item.id === manualPresetSelectedId.value)
  const saved: ManualPreset = {
    id: existing?.id || newManualPresetID(),
    name,
    settings: currentManualPresetSettings(),
    created_at: existing?.created_at || now,
    updated_at: now,
  }
  manualPresets.value = [
    saved,
    ...manualPresets.value.filter((item) => item.id !== saved.id),
  ].slice(0, 50)
  manualPresetSelectedId.value = saved.id
  manualPresetName.value = saved.name
  persistManualPresets()
  appStore.showSuccess(t('admin.imageChannelMonitor.manual.presetSaved'))
}

function deleteManualPreset() {
  const id = manualPresetSelectedId.value
  if (!id) return
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

function resolvedManualSize() {
  switch (manualForm.size_mode) {
    case 'auto':
      return 'auto'
    case 'preset':
      return manualForm.size.trim()
    case 'custom':
      return manualForm.custom_size.trim()
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

  manualRunning.value = true
  manualResults.value = Object.fromEntries(
    selectedTargets.map((target) => [
      target.id,
      {
        monitor: target,
        state: 'running',
        message: t('admin.imageChannelMonitor.manual.requesting'),
      } satisfies ManualResultItem,
    ])
  )

  const payload = {
    mode: manualForm.mode,
    model: manualForm.model,
    prompt: manualForm.prompt,
    size: resolvedManualSize(),
    quality: manualForm.quality,
    n: manualForm.n,
    download_image: manualForm.download_image,
    timeout_seconds: manualForm.timeout_seconds,
    input_image_data: manualForm.mode === 'edit' ? manualInputImage.value?.data : undefined,
    input_image_type: manualForm.mode === 'edit' ? manualInputImage.value?.type : undefined,
    input_image_name: manualForm.mode === 'edit' ? manualInputImage.value?.name : undefined,
  }

  try {
    await Promise.allSettled(
      selectedTargets.map(async (target) => {
        try {
          const run = await adminAPI.imageChannelMonitor.manualTest(target.id, payload)
          if (manualRunSeq !== seq) return
          setManualResultFromRun(target, run)
          if (run.running) {
            await pollManualRun(target, run.run_id, payload.timeout_seconds, seq)
          }
        } catch (err: unknown) {
          if (manualRunSeq !== seq) return
          setManualResult(target.id, {
            monitor: target,
            state: 'error',
            message: extractApiErrorMessage(err, t('admin.imageChannelMonitor.manual.failed')),
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
      })
      return
    }
  }
  if (manualRunSeq !== seq) return
  setManualResult(target.id, {
    monitor: target,
    state: 'error',
    message: t('admin.imageChannelMonitor.manual.failed'),
  })
}

function setManualResultFromRun(target: ImageChannelMonitor, run: ImageChannelManualRunResponse) {
  setManualResult(target.id, {
    monitor: run.monitor || target,
    state: run.running ? 'running' : run.result ? 'done' : 'error',
    message: run.message || run.result?.message || '',
    run,
  })
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

function manualResultBadgeText(item: ManualResultItem) {
  if (item.state === 'running') return t('admin.imageChannelMonitor.manual.running')
  if (item.state === 'error') return t('admin.imageChannelMonitor.manual.error')
  const status = manualRunResult(item)?.status
  return status ? statusLabel(status) : t('admin.imageChannelMonitor.manual.done')
}

function manualResultBadgeClass(item: ManualResultItem) {
  if (item.state === 'running') {
    return 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200'
  }
  if (item.state === 'error') {
    return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  }
  return statusBadgeClass(manualRunResult(item)?.status || 'error')
}

function manualResultStatusText(item: ManualResultItem) {
  if (item.state === 'running') {
    const stage = item.run?.stage ? t(`admin.imageChannelMonitor.stages.${item.run.stage}`, item.run.stage) : ''
    return [stage, item.message].filter(Boolean).join(' / ')
  }
  if (item.state === 'error') return item.message
  const result = manualRunResult(item)
  if (!result) return ''
  const httpStatus = result.http_status ? `HTTP ${result.http_status}` : ''
  const stage = result.error_stage || result.stages?.at(-1)?.stage || ''
  const stageText = stage ? t(`admin.imageChannelMonitor.stages.${stage}`, stage) : ''
  return [httpStatus, stageText].filter(Boolean).join(' / ')
}

function manualRunResult(item: ManualResultItem) {
  return item.run?.result
}

function manualPreview(item: ManualResultItem) {
  const result = manualRunResult(item)
  return result?.returned_image_url || result?.returned_image_data || ''
}

function formatMs(value: number | null) {
  return typeof value === 'number' ? `${value} ms` : '-'
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

onMounted(() => {
  loadManualPresets()
  reload()
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
})
</script>
