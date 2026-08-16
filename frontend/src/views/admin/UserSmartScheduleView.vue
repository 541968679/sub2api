<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <button
            type="button"
            class="mb-2 inline-flex items-center text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
            data-testid="smart-schedule-back"
            @click="goBack"
          >
            <Icon name="arrowLeft" size="sm" class="mr-1" />
            {{ t('admin.users.smartSchedule.backToUsers') }}
          </button>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.users.smartSchedule.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ userLoading ? t('common.loading') : userSubtitle }}
          </p>
        </div>
        <div
          v-if="currentDraft && !loading && !userLoading"
          class="flex items-center gap-3 rounded-2xl border px-4 py-3 shadow-sm"
          :class="
            currentDraft.enabled
              ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/40'
              : 'border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800'
          "
          data-testid="smart-schedule-enable-card"
        >
          <div class="text-right">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">
              {{
                currentDraft.enabled
                  ? t('admin.users.smartSchedule.enabledOn')
                  : t('admin.users.smartSchedule.enabledOff')
              }}
              · {{ platformLabel(activePlatform) }}
            </p>
            <p class="mt-0.5 max-w-[16rem] text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.users.smartSchedule.enabledSwitchHint') }}
            </p>
          </div>
          <button
            type="button"
            role="switch"
            class="relative inline-flex h-9 w-16 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:ring-offset-dark-800"
            :class="currentDraft.enabled ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'"
            :aria-checked="currentDraft.enabled"
            :disabled="submitting"
            data-testid="smart-schedule-enabled"
            @click="onToggleEnabled"
          >
            <span
              class="pointer-events-none inline-block h-8 w-8 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="currentDraft.enabled ? 'translate-x-7' : 'translate-x-0'"
            />
          </button>
        </div>
      </div>

      <div v-if="loading || userLoading" class="py-16 text-center text-gray-500">
        {{ t('common.loading') }}
      </div>

      <div
        v-else
        class="flex flex-col gap-4 lg:flex-row lg:items-start"
        data-testid="smart-schedule-layout"
      >
        <aside
          class="w-full shrink-0 space-y-3 rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800 lg:w-[26rem] xl:w-[28rem]"
          data-testid="smart-schedule-user-panel"
        >
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-700/60">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
              {{ user?.email || t('admin.users.smartSchedule.unknownUser') }}
            </p>
            <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
              {{ userMeta }}
            </p>
          </div>

          <div data-testid="smart-schedule-tabs">
            <p class="mb-1.5 text-xs font-medium uppercase tracking-wide text-gray-400">
              {{ t('admin.users.smartSchedule.platform') }}
            </p>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="platform in platforms"
                :key="platform"
                type="button"
                class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium"
                :class="
                  activePlatform === platform
                    ? 'bg-primary-500 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'
                "
                :data-testid="`smart-schedule-tab-${platform}`"
                @click="activePlatform = platform"
              >
                <span>{{ platformLabel(platform) }}</span>
                <span
                  v-if="drafts[platform]?.enabled"
                  class="h-1.5 w-1.5 rounded-full"
                  :class="activePlatform === platform ? 'bg-white' : 'bg-emerald-500'"
                />
              </button>
            </div>
          </div>

          <div v-if="currentDraft" class="space-y-3">
            <div class="grid grid-cols-2 gap-2">
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityMaxP50') }}</span>
                <input
                  v-model.number="currentDraft.maxP50"
                  type="number"
                  min="1"
                  class="input w-full"
                  data-testid="smart-schedule-p50"
                />
              </label>
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityMinSuccess') }}</span>
                <input
                  v-model.number="currentDraft.successPercent"
                  type="number"
                  min="0"
                  max="100"
                  step="0.1"
                  class="input w-full"
                  data-testid="smart-schedule-success"
                />
              </label>
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityMinSuccessSamples') }}</span>
                <input v-model.number="currentDraft.minSuccessSamples" type="number" min="1" class="input w-full" />
              </label>
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityMinTtftSamples') }}</span>
                <input v-model.number="currentDraft.minTtftSamples" type="number" min="1" class="input w-full" />
              </label>
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.users.smartSchedule.cooldown') }}</span>
                <input
                  v-model.number="currentDraft.cooldownMinutes"
                  type="number"
                  min="1"
                  max="1440"
                  class="input w-full"
                  data-testid="smart-schedule-cooldown"
                />
              </label>
              <label class="space-y-1">
                <span class="block text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityConditionOr') }}</span>
                <select v-model="currentDraft.condition" class="input w-full">
                  <option value="or">{{ t('admin.accounts.userSchedule.qualityConditionOr') }}</option>
                  <option value="and">{{ t('admin.accounts.userSchedule.qualityConditionAnd') }}</option>
                </select>
              </label>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="qualityTemplateBusy"
                @click="applyQualityTemplate(applyTemplateToDraft)"
              >
                {{ t('admin.accounts.stability.applyTemplate') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="qualityTemplateBusy"
                @click="saveQualityTemplate(qualityGateFormFromDraft(currentDraft))"
              >
                {{ t('admin.accounts.stability.saveTemplate') }}
              </button>
              <select v-model="copyFromPlatform" class="input min-w-[8rem] flex-1" data-testid="smart-schedule-copy-from">
                <option value="">{{ t('admin.users.smartSchedule.copyFrom') }}</option>
                <option v-for="platform in otherPlatforms" :key="platform" :value="platform">
                  {{ platformLabel(platform) }}
                </option>
              </select>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="!copyFromPlatform || copying"
                data-testid="smart-schedule-copy"
                @click="onCopy"
              >
                {{ t('admin.users.smartSchedule.copy') }}
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="submitting || loading"
                data-testid="smart-schedule-save"
                @click="onSave"
              >
                {{ submitting ? t('admin.users.smartSchedule.saving') : t('admin.users.smartSchedule.save') }}
              </button>
            </div>
          </div>
        </aside>

        <section
          class="min-w-0 flex-1 space-y-4 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
          data-testid="smart-schedule-pool-panel"
        >
          <div class="flex flex-wrap items-end justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.users.smartSchedule.poolTitle', { platform: platformLabel(activePlatform) }) }}
                </h2>
                <div class="relative" ref="columnDropdownRef">
                  <button
                    type="button"
                    class="btn btn-secondary px-2 md:px-3"
                    :title="t('admin.users.columnSettings')"
                    data-testid="smart-schedule-column-settings"
                    @click="showColumnDropdown = !showColumnDropdown"
                  >
                    <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
                    </svg>
                    <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
                  </button>
                  <div
                    v-if="showColumnDropdown"
                    class="absolute left-0 z-50 mt-2 w-72 origin-top-left rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
                    data-testid="smart-schedule-column-settings-menu"
                  >
                    <div class="border-b border-gray-100 px-3 py-2 dark:border-gray-700">
                      <p class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.accounts.columnLayoutHint') }}
                      </p>
                      <button
                        type="button"
                        class="mt-1.5 text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                        data-testid="smart-schedule-column-layout-reset"
                        @click="resetColumnLayout"
                      >
                        {{ t('admin.accounts.resetColumnLayout') }}
                      </button>
                    </div>
                    <div class="max-h-80 overflow-y-auto p-2">
                      <div
                        v-for="col in orderedToggleableColumns"
                        :key="col.key"
                        class="flex items-center gap-1 rounded-md px-1 py-1 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
                        data-testid="smart-schedule-column-settings-row"
                        :data-column-key="col.key"
                      >
                        <button
                          type="button"
                          class="min-w-0 flex-1 rounded-md px-2 py-1.5 text-left"
                          @click="toggleColumn(col.key)"
                        >
                          <span class="flex items-center justify-between gap-2">
                            <span class="truncate">{{ col.label }}</span>
                            <Icon
                              v-if="isColumnVisible(col.key)"
                              name="check"
                              size="sm"
                              class="shrink-0 text-primary-500"
                            />
                          </span>
                        </button>
                        <div class="flex shrink-0 flex-col">
                          <button
                            type="button"
                            class="rounded p-0.5 text-gray-400 hover:bg-gray-200 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-gray-600 dark:hover:text-gray-100"
                            :disabled="!canMoveColumn(col.key, 'up')"
                            :title="t('admin.accounts.moveColumnUp')"
                            :data-testid="`smart-schedule-column-move-up-${col.key}`"
                            @click="moveColumn(col.key, 'up')"
                          >
                            <Icon name="chevronUp" size="xs" :stroke-width="2" />
                          </button>
                          <button
                            type="button"
                            class="rounded p-0.5 text-gray-400 hover:bg-gray-200 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-gray-600 dark:hover:text-gray-100"
                            :disabled="!canMoveColumn(col.key, 'down')"
                            :title="t('admin.accounts.moveColumnDown')"
                            :data-testid="`smart-schedule-column-move-down-${col.key}`"
                            @click="moveColumn(col.key, 'down')"
                          >
                            <Icon name="chevronDown" size="xs" :stroke-width="2" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.users.smartSchedule.poolHint') }}
              </p>
            </div>
            <div class="flex flex-col items-stretch gap-3">
              <div class="flex flex-wrap items-end gap-2">
                <label class="relative min-w-[16rem] flex-1 space-y-1">
                  <span class="block text-xs text-gray-500">{{ t('admin.users.smartSchedule.addAccount') }}</span>
                  <input
                    v-model="accountSearchQuery"
                    type="text"
                    class="input w-full"
                    data-testid="smart-schedule-add-select"
                    :placeholder="t('admin.users.smartSchedule.addAccountPlaceholder')"
                    autocomplete="off"
                    @focus="accountSearchOpen = true"
                    @input="accountSearchOpen = true"
                    @blur="onAccountSearchBlur"
                  />
                  <div
                    v-if="accountSearchOpen"
                    class="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
                    data-testid="smart-schedule-add-dropdown"
                    @mousedown.prevent
                  >
                    <button
                      v-for="account in filteredAddableAccounts"
                      :key="account.id"
                      type="button"
                      class="block w-full px-3 py-2 text-left text-sm text-gray-800 hover:bg-gray-100 dark:text-gray-100 dark:hover:bg-dark-700"
                      :data-testid="`smart-schedule-add-option-${account.id}`"
                      @click="chooseAddableAccount(account.id)"
                    >
                      {{ account.name }} (#{{ account.id }})
                    </button>
                    <p
                      v-if="filteredAddableAccounts.length === 0"
                      class="px-3 py-2 text-sm text-gray-500"
                    >
                      {{ t('admin.users.smartSchedule.addAccountEmpty') }}
                    </p>
                  </div>
                </label>
                <label class="space-y-1">
                  <span class="block text-xs text-gray-500">{{ t('admin.users.smartSchedule.applyCap') }}</span>
                  <input v-model.number="bulkCap" type="number" min="1" class="input w-24" />
                </label>
                <button type="button" class="btn btn-secondary btn-sm" data-testid="smart-schedule-apply-cap" @click="applyCapToAll">
                  {{ t('admin.users.smartSchedule.applyCapButton') }}
                </button>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-xs text-gray-500">{{ t('admin.users.smartSchedule.addScheduling') }}</span>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="addableSchedulingApi.length === 0"
                  data-testid="smart-schedule-add-api"
                  @click="addSchedulingAccounts('apikey')"
                >
                  {{ t('admin.users.smartSchedule.addSchedulingApi', { count: addableSchedulingApi.length }) }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="addableSchedulingOauth.length === 0"
                  data-testid="smart-schedule-add-oauth"
                  @click="addSchedulingAccounts('oauth')"
                >
                  {{ t('admin.users.smartSchedule.addSchedulingOauth', { count: addableSchedulingOauth.length }) }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="addableSchedulingAll.length === 0"
                  data-testid="smart-schedule-add-all"
                  @click="addSchedulingAccounts('all')"
                >
                  {{ t('admin.users.smartSchedule.addSchedulingAll', { count: addableSchedulingAll.length }) }}
                </button>
              </div>
            </div>
          </div>

          <p v-if="emptyPoolError" class="text-sm text-red-600" data-testid="smart-schedule-empty-error">
            {{ t('admin.users.smartSchedule.emptyPool') }}
          </p>

          <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
            <DataTable
              :columns="poolColumns"
              :data="poolTableRows"
              :loading="statsLoading"
              row-key="id"
              :virtual-scroll="false"
              :resizable-columns="true"
              default-sort-key="name"
              default-sort-order="asc"
              sort-storage-key="smart-schedule-pool-sort"
              data-testid="smart-schedule-pool-table"
              @column-resize="handleColumnResize"
            >
              <template #empty>
                <p class="py-6 text-center text-sm text-gray-500">
                  {{ t('admin.users.smartSchedule.emptyPoolHint') }}
                </p>
              </template>
              <template #cell-name="{ row, value }">
                <div class="flex flex-col">
                  <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
                  <span
                    v-if="getAccountEmail(row)"
                    class="max-w-[200px] truncate text-xs text-gray-500 dark:text-gray-400"
                    :title="getAccountEmail(row)"
                  >
                    {{ getAccountEmail(row) }}
                  </span>
                </div>
              </template>
              <template #cell-platform_type="{ row }">
                <PlatformTypeBadge
                  :platform="row.platform"
                  :type="row.type"
                  :plan-type="row.credentials?.plan_type || row.parent_plan_type"
                  :privacy-mode="row.extra?.privacy_mode || row.parent_privacy_mode"
                  :subscription-expires-at="row.credentials?.subscription_expires_at || row.parent_subscription_expires_at"
                />
              </template>
              <template #header-concurrency="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.inlineEdit.capacityConcurrencyHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-concurrency="{ row }">
                <div class="flex flex-col gap-0.5">
                  <AccountInlineNumberCell
                    :model-value="row.concurrency"
                    :min="1"
                    :disabled="inlineSavingId === row.id"
                    :hint="t('admin.accounts.inlineEdit.concurrencyHint')"
                    @save="(value) => handleInlineConcurrency(row, value)"
                  />
                  <AccountCapacityCell :account="row" />
                </div>
              </template>
              <template #header-pair_cap="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.pairCapHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-pair_cap="{ row }">
                <div class="flex flex-col gap-0.5">
                  <AccountInlineNumberCell
                    :model-value="memberCap(row.id)"
                    :min="0"
                    :blank-when-zero="true"
                    :hint="t('admin.users.smartSchedule.pairCapHint')"
                    @save="(value) => setMemberCap(row.id, value)"
                  />
                  <CapacityBadge
                    :color-class="pairCapacityClass(memberCurrent(row.id), effectivePairMax(row))"
                    :current="memberCurrent(row.id)"
                    :max="effectivePairMax(row) || '—'"
                  />
                </div>
              </template>
              <template #cell-status="{ row }">
                <AccountStatusIndicator :account="row" @show-temp-unsched="handleShowTempUnsched" />
              </template>
              <template #header-schedulable="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.schedulableColumnHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-schedulable="{ row }">
                <div class="flex flex-col gap-1">
                  <div class="flex items-center gap-1.5">
                    <button
                      type="button"
                      :disabled="togglingSchedulable === row.id"
                      class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-dark-800"
                      :class="[row.schedulable ? 'bg-primary-500 hover:bg-primary-600' : 'bg-gray-200 hover:bg-gray-300 dark:bg-dark-600 dark:hover:bg-dark-500']"
                      :title="row.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled')"
                      data-testid="account-schedulable-toggle"
                      @click="handleToggleSchedulable(row)"
                    >
                      <span
                        class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                        :class="[row.schedulable ? 'translate-x-4' : 'translate-x-0']"
                      />
                    </button>
                    <span class="text-[10px] leading-none text-gray-500 dark:text-gray-400">{{ t('admin.accounts.columns.schedulable') }}</span>
                  </div>
                  <div class="flex items-center gap-1.5">
                    <button
                      type="button"
                      :disabled="inlineSavingId === row.id"
                      class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-dark-800"
                      :class="[isFallbackOnly(row) ? 'bg-amber-500 hover:bg-amber-600' : 'bg-gray-200 hover:bg-gray-300 dark:bg-dark-600 dark:hover:bg-dark-500']"
                      :title="isFallbackOnly(row) ? t('admin.accounts.fallbackOnly') : t('admin.accounts.fallbackOnlyHint')"
                      data-testid="account-fallback-toggle"
                      @click="handleToggleFallbackOnly(row)"
                    >
                      <span
                        class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                        :class="[isFallbackOnly(row) ? 'translate-x-4' : 'translate-x-0']"
                      />
                    </button>
                    <span class="text-[10px] leading-none text-gray-500 dark:text-gray-400">{{ t('admin.accounts.fallbackOnlyShort') }}</span>
                  </div>
                </div>
              </template>
              <template #header-quality_ttft="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.quality.combinedHint')" width-class="w-80" />
                </div>
              </template>
              <template #cell-quality_ttft="{ row }">
                <AccountQualityCell
                  clickable
                  mode="combined"
                  :stats="qualityStatsById[String(row.id)] ?? null"
                  :loading="statsLoading"
                  @click="openStabilityDialog(row)"
                />
              </template>
              <template #cell-today_stats="{ row }">
                <AccountTodayStatsCell
                  :stats="todayStatsById[String(row.id)] ?? null"
                  :loading="statsLoading"
                />
              </template>
              <template #cell-groups="{ row }">
                <AccountGroupsCell :groups="row.groups" :max-display="4" />
              </template>
              <template #header-usage="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.usageWindowsHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-usage="{ row }">
                <AccountUsageCell
                  :account="row"
                  :today-stats="todayStatsById[String(row.id)] ?? null"
                  :today-stats-loading="statsLoading"
                  @account-updated="handleAccountUpdated"
                />
              </template>
              <template #header-priority="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.priorityHint')" width-class="w-64" />
                </div>
              </template>
              <template #cell-priority="{ row }">
                <AccountInlineNumberCell
                  :model-value="row.priority ?? 0"
                  :min="0"
                  :disabled="inlineSavingId === row.id"
                  :hint="t('admin.accounts.priorityHint')"
                  @save="(value) => handleInlinePriority(row, value)"
                />
              </template>
              <template #header-upstream_rate_multiplier="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.accounts.upstreamRateMultiplierHint')" width-class="w-64" />
                </div>
              </template>
              <template #cell-upstream_rate_multiplier="{ row }">
                <AccountInlineNumberCell
                  :model-value="row.upstream_rate_multiplier ?? 1"
                  :min="0"
                  :step="0.01"
                  :allow-decimal="true"
                  :disabled="inlineSavingId === row.id"
                  :hint="t('admin.accounts.upstreamRateMultiplierHint')"
                  @save="(value) => handleInlineUpstreamRate(row, value)"
                />
              </template>
              <template #cell-last_used_at="{ value }">
                <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatRelativeTime(value) }}</span>
              </template>
              <template #cell-actions="{ row }">
                <div class="flex flex-wrap items-center gap-1">
                  <button
                    type="button"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                    @click="handleEdit(row)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0115.75 21H5.25A2.25 2.25 0 013 18.75V8.25A2.25 2.25 0 015.25 6H10" />
                    </svg>
                    <span class="text-xs">{{ t('common.edit') }}</span>
                  </button>
                  <button
                    type="button"
                    data-testid="account-open-stability"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-900/20 dark:hover:text-amber-400"
                    :title="t('admin.accounts.viewStability')"
                    @click="openStabilityDialog(row)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h.01M9.75 8.625c0-.621.504-1.125 1.125-1.125h.01M16.5 4.125c0-.621.504-1.125 1.125-1.125h.01M3 19.875v-6.75M9.75 19.875V8.625M16.5 19.875V4.125M21 19.875H3" />
                    </svg>
                    <span class="text-xs">{{ t('admin.accounts.viewStabilityShort') }}</span>
                  </button>
                  <button
                    type="button"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                    :title="t('admin.accounts.viewUsage')"
                    @click="handleViewUsage(row)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
                    </svg>
                    <span class="text-xs">{{ t('admin.accounts.viewUsageShort') }}</span>
                  </button>
                  <button
                    type="button"
                    data-testid="account-view-error-requests"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-900/20 dark:hover:text-amber-400"
                    :title="t('admin.accounts.viewErrorRequests')"
                    @click="handleViewErrorRequests(row)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
                    </svg>
                    <span class="text-xs">{{ t('admin.accounts.viewErrorRequestsShort') }}</span>
                  </button>
                  <button
                    type="button"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-emerald-50 hover:text-emerald-600 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400"
                    :title="t('admin.users.smartSchedule.resume')"
                    @click="resumePair(row.id)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
                    </svg>
                    <span class="text-xs">{{ t('admin.users.smartSchedule.resume') }}</span>
                  </button>
                  <button
                    type="button"
                    data-testid="smart-schedule-remove"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                    :title="t('admin.users.smartSchedule.removeFromPool')"
                    @click="removeAccount(row.id)"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M15 12H9m12 0a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <span class="text-xs">{{ t('admin.users.smartSchedule.removeFromPool') }}</span>
                  </button>
                  <button
                    type="button"
                    class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:hover:bg-dark-700 dark:hover:text-white"
                    @click="openMenu(row, $event)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6.75 12a.75.75 0 11-1.5 0 .75.75 0 011.5 0zM12.75 12a.75.75 0 11-1.5 0 .75.75 0 011.5 0zM18.75 12a.75.75 0 11-1.5 0 .75.75 0 011.5 0z" />
                    </svg>
                    <span class="text-xs">{{ t('common.more') }}</span>
                  </button>
                </div>
              </template>
            </DataTable>
          </div>
        </section>
      </div>
    </div>
    <EditAccountModal
      :show="showEdit"
      :account="edAcc"
      :proxies="proxies"
      :groups="groups"
      @close="showEdit = false"
      @updated="handleAccountUpdated"
    />
    <ReAuthAccountModal :show="showReAuth" :account="reAuthAcc" @close="showReAuth = false" @reauthorized="handleAccountUpdated" />
    <UpdateRefreshTokenModal :show="showUpdateRt" :account="updateRtAcc" @close="showUpdateRt = false" @updated="handleAccountUpdated" />
    <AccountTestModal :show="showTest" :account="testingAcc" @close="showTest = false" />
    <AccountStatsModal :show="showStats" :account="statsAcc" @close="showStats = false" />
    <ScheduledTestsPanel
      :show="showSchedulePanel"
      :account-id="scheduleAcc?.id ?? null"
      :model-options="scheduleModelOptions"
      @close="showSchedulePanel = false"
    />
    <AccountActionMenu
      :show="menu.show"
      :account="menu.acc"
      :position="menu.pos"
      @close="menu.show = false"
      @test="handleTest"
      @stats="handleViewStats"
      @schedule="handleSchedule"
      @reauth="handleReAuth"
      @refresh-token="handleRefresh"
      @update-refresh-token="handleUpdateRefreshToken"
      @recover-state="handleRecoverState"
      @reset-quota="handleResetQuota"
      @set-privacy="handleSetPrivacy"
      @export-codex="handleExportCodexAuth"
      @create-spark-shadow="handleCreateSparkShadow"
      @clear-stuck-runtime="handleClearStuckRuntime"
    />
    <AccountStabilityDialog
      :show="showStability"
      :account="stabilityAcc"
      @close="showStability = false"
      @recovered="handleStabilityRecovered"
    />
    <TempUnschedStatusModal
      :show="showTempUnsched"
      :account="tempUnschedAcc"
      @close="showTempUnsched = false"
      @reset="handleTempUnschedReset"
    />
    <ConfirmDialog
      :show="showSparkShadowDialog"
      :title="t('admin.accounts.createSparkShadow')"
      :message="t('admin.accounts.createSparkShadowConfirm', { name: sparkShadowParent?.name || '' })"
      :confirm-text="t('common.confirm')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmCreateSparkShadow"
      @cancel="cancelCreateSparkShadow"
    />
    <UsageErrorInspectDialog
      :show="inspectOpen"
      scope="account"
      :subject-id="inspectSubjectId"
      :subject-label="inspectSubjectLabel"
      :initial-tab="inspectTab"
      @close="inspectOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatRelativeTime } from '@/utils/format'
import { useAppStore } from '@/stores/app'
import { useUserSmartScheduleEditor } from '@/composables/useUserSmartScheduleEditor'
import { useSmartSchedulePoolAccountOps } from '@/composables/useSmartSchedulePoolAccountOps'
import { useSmartSchedulePoolColumnLayout } from '@/composables/useSmartSchedulePoolColumnLayout'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DataTable from '@/components/common/DataTable.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountQualityCell from '@/components/account/AccountQualityCell.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import AccountInlineNumberCell from '@/components/account/AccountInlineNumberCell.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import CapacityBadge from '@/components/account/CapacityBadge.vue'
import AccountStabilityDialog from '@/components/account/AccountStabilityDialog.vue'
import { EditAccountModal, TempUnschedStatusModal } from '@/components/account'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import ReAuthAccountModal from '@/components/admin/account/ReAuthAccountModal.vue'
import UpdateRefreshTokenModal from '@/components/admin/account/UpdateRefreshTokenModal.vue'
import AccountTestModal from '@/components/admin/account/AccountTestModal.vue'
import AccountStatsModal from '@/components/admin/account/AccountStatsModal.vue'
import ScheduledTestsPanel from '@/components/admin/account/ScheduledTestsPanel.vue'
import UsageErrorInspectDialog from '@/components/admin/usage/UsageErrorInspectDialog.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const user = ref<AdminUser | null>(null)
const userLoading = ref(false)
const accountSearchQuery = ref('')
const accountSearchOpen = ref(false)

const userId = computed(() => {
  const raw = route.params.id
  const value = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(value) && value > 0 ? value : null
})

const {
  platforms,
  loading,
  submitting,
  copying,
  statsLoading,
  emptyPoolError,
  activePlatform,
  copyFromPlatform,
  bulkCap,
  drafts,
  poolAccounts,
  qualityStatsById,
  todayStatsById,
  currentDraft,
  otherPlatforms,
  addableAccounts,
  addableSchedulingApi,
  addableSchedulingOauth,
  addableSchedulingAll,
  qualityTemplateBusy,
  applyQualityTemplate,
  saveQualityTemplate,
  applyTemplateToDraft,
  qualityGateFormFromDraft,
  memberCap,
  memberCurrent,
  effectivePairMax,
  setMemberCap,
  patchPoolAccount,
  applyCapToAll,
  addAccountById,
  addSchedulingAccounts,
  removeAccount,
  onToggleEnabled,
  onSave,
  onCopy,
  resumePair
} = useUserSmartScheduleEditor(userId)

const {
  groups,
  proxies,
  inlineSavingId,
  togglingSchedulable,
  isFallbackOnly,
  showEdit,
  edAcc,
  showStability,
  stabilityAcc,
  showTempUnsched,
  tempUnschedAcc,
  showTest,
  testingAcc,
  showStats,
  statsAcc,
  showReAuth,
  reAuthAcc,
  showUpdateRt,
  updateRtAcc,
  showSchedulePanel,
  scheduleAcc,
  scheduleModelOptions,
  showSparkShadowDialog,
  sparkShadowParent,
  inspectOpen,
  inspectSubjectId,
  inspectSubjectLabel,
  inspectTab,
  menu,
  handleInlineConcurrency,
  handleInlinePriority,
  handleInlineUpstreamRate,
  handleToggleSchedulable,
  handleToggleFallbackOnly,
  handleEdit,
  handleAccountUpdated,
  openStabilityDialog,
  handleStabilityRecovered,
  handleShowTempUnsched,
  handleTempUnschedReset,
  handleViewUsage,
  handleViewErrorRequests,
  openMenu,
  handleTest,
  handleViewStats,
  handleSchedule,
  handleReAuth,
  handleUpdateRefreshToken,
  handleRefresh,
  handleRecoverState,
  handleResetQuota,
  handleClearStuckRuntime,
  handleSetPrivacy,
  handleExportCodexAuth,
  handleCreateSparkShadow,
  cancelCreateSparkShadow,
  confirmCreateSparkShadow
} = useSmartSchedulePoolAccountOps({ patchPoolAccount })

const allPoolColumns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.accounts.columns.name'), sortable: true, minWidth: 140 },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType'), sortable: true, minWidth: 120 },
  { key: 'concurrency', label: t('admin.accounts.columns.capacity'), sortable: true, minWidth: 88 },
  { key: 'pair_cap', label: t('admin.users.smartSchedule.pairCap'), sortable: true, minWidth: 88 },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: true, minWidth: 88 },
  { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: true, minWidth: 88 },
  { key: 'quality_ttft', label: t('admin.accounts.columns.quality'), sortable: false, minWidth: 88 },
  { key: 'today_stats', label: t('admin.accounts.columns.todayStats'), sortable: false, minWidth: 88 },
  { key: 'groups', label: t('admin.accounts.columns.groups'), sortable: false, minWidth: 88 },
  { key: 'usage', label: t('admin.accounts.columns.usageWindows'), sortable: false, minWidth: 88 },
  { key: 'priority', label: t('admin.accounts.columns.priority'), sortable: true, minWidth: 72 },
  { key: 'upstream_rate_multiplier', label: t('admin.accounts.columns.upstreamRateMultiplier'), sortable: true, minWidth: 88 },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: true, minWidth: 88 },
  { key: 'actions', label: t('admin.accounts.columns.actions'), sortable: false, minWidth: 280, resizable: false }
])

const {
  showColumnDropdown,
  columnDropdownRef,
  orderedToggleableColumns,
  visibleColumns: poolColumns,
  isColumnVisible,
  toggleColumn,
  canMoveColumn,
  moveColumn,
  handleColumnResize,
  resetColumnLayout
} = useSmartSchedulePoolColumnLayout(allPoolColumns)

const poolTableRows = computed(() =>
  poolAccounts.value.map((account) => ({
    ...account,
    platform_type: `${account.platform} ${account.type}`,
    pair_cap: memberCap(account.id),
    pair_current: memberCurrent(account.id)
  }))
)

function getAccountEmail(row: Account): string | undefined {
  const email = row.extra?.email_address || row.credentials?.email || row.parent_email
  return typeof email === 'string' ? email : undefined
}

function pairCapacityClass(current: number, max: number) {
  if (max > 0 && current >= max) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (current > 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
}

const userSubtitle = computed(() => {
  if (!user.value) return t('admin.users.smartSchedule.pageDescription')
  return t('admin.users.smartSchedule.subtitle', { email: user.value.email })
})

const userMeta = computed(() => {
  if (!user.value) return ''
  const parts = [`#${user.value.id}`]
  if (user.value.username) parts.push(user.value.username)
  parts.push(t(`admin.users.${user.value.role === 'admin' ? 'admin' : 'user'}`))
  return parts.join(' · ')
})

const filteredAddableAccounts = computed(() => {
  const query = accountSearchQuery.value.trim().toLowerCase()
  const source = addableAccounts.value
  if (!query) return source.slice(0, 30)
  return source
    .filter((account) => {
      return account.name.toLowerCase().includes(query) || String(account.id).includes(query)
    })
    .slice(0, 50)
})

function chooseAddableAccount(accountId: number) {
  addAccountById(accountId)
  accountSearchQuery.value = ''
  accountSearchOpen.value = false
}

function onAccountSearchBlur() {
  window.setTimeout(() => {
    accountSearchOpen.value = false
  }, 120)
}

function platformLabel(platform: string) {
  return t(`admin.groups.platforms.${platform}`)
}

function goBack() {
  void router.push({ name: 'AdminUsers' })
}

async function loadUser() {
  if (!userId.value) return
  userLoading.value = true
  try {
    user.value = await adminAPI.users.getById(userId.value)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
  } finally {
    userLoading.value = false
  }
}

onMounted(() => {
  void loadUser()
})
</script>
