<template>
  <AppLayout>
    <div class="space-y-3">
      <div class="flex flex-wrap items-center gap-3" data-testid="smart-schedule-page-header">
        <button
          type="button"
          class="inline-flex items-center text-sm text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
          data-testid="smart-schedule-back"
          @click="goBack"
        >
          <Icon name="arrowLeft" size="sm" class="mr-1" />
          {{ t('admin.users.smartSchedule.backToUsers') }}
        </button>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.users.smartSchedule.pageDescription') }}
        </p>
      </div>

      <div v-if="!pageReady" class="py-10 text-center text-gray-500" data-testid="smart-schedule-page-loading">
        {{ t('common.loading') }}
      </div>

      <template v-else>
      <div v-if="user" data-testid="smart-schedule-user-row">
        <AdminUserListRowTable
          :user="user"
          :groups="groups"
          :quality-stats="userQualityStats"
          :quality-loading="userQualityLoading"
          :quality-error="userQualityError"
          :window-n="accountQualityWindowN"
          :usage-stats="userUsageStats"
          :burn-rate-stats="userBurnRateStats"
          :smart-schedule="userSmartScheduleSummary"
          :smart-schedule-loading="false"
          :schedule-pnl="userSchedulePnl"
          :schedule-pnl-loading="userSchedulePnlLoading"
          @updated="loadUser({ silent: true })"
          @deleted="goBack"
          @open-schedule-pnl="openUserSchedulePnl"
          @open-user-quality="openUserQuality"
        />
      </div>
        <div
          class="grid grid-cols-1 items-stretch gap-2 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]"
          data-testid="smart-schedule-layout"
        >
        <aside
          class="flex h-full min-w-0 flex-col gap-1.5 rounded-xl border border-gray-200 bg-white px-2 py-1.5 dark:border-dark-700 dark:bg-dark-800"
          data-testid="smart-schedule-user-panel"
        >
          <div
            class="flex flex-wrap items-center gap-2"
            data-testid="smart-schedule-threshold-toolbar"
          >
            <p class="min-w-0 truncate rounded-md bg-gray-50 px-2 py-1 text-sm font-medium text-gray-900 dark:bg-dark-700/60 dark:text-white">
              {{ user?.email || t('admin.users.smartSchedule.unknownUser') }}
              <span v-if="userMeta" class="ml-1.5 text-xs font-normal text-gray-500 dark:text-gray-400">
                {{ userMeta }}
              </span>
            </p>
            <div
              v-if="currentDraft"
              class="flex shrink-0 items-center gap-1.5 rounded-lg border px-2 py-1"
              :class="
                currentDraft.enabled
                  ? 'border-emerald-200 bg-emerald-50 dark:border-emerald-800 dark:bg-emerald-950/40'
                  : 'border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800'
              "
              data-testid="smart-schedule-enable-card"
            >
              <p class="text-xs font-semibold text-gray-900 dark:text-white">
                {{
                  currentDraft.enabled
                    ? t('admin.users.smartSchedule.enabledOn')
                    : t('admin.users.smartSchedule.enabledOff')
                }}
                · {{ platformLabel(activePlatform) }}
              </p>
              <button
                type="button"
                role="switch"
                class="relative inline-flex h-6 w-10 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 dark:focus:ring-offset-dark-800"
                :class="currentDraft.enabled ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'"
                :aria-checked="currentDraft.enabled"
                :disabled="submitting"
                :title="t('admin.users.smartSchedule.enabledSwitchHint')"
                data-testid="smart-schedule-enabled"
                @click="onToggleEnabled"
              >
                <span
                  class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                  :class="currentDraft.enabled ? 'translate-x-4' : 'translate-x-0'"
                />
              </button>
            </div>
            <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1" data-testid="smart-schedule-tabs">
              <span class="shrink-0 text-xs text-gray-400">
                {{ t('admin.users.smartSchedule.platform') }}
              </span>
              <button
                v-for="platform in platforms"
                :key="platform"
                type="button"
                class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium"
                :class="
                  activePlatform === platform
                    ? 'bg-primary-500 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'
                "
                :data-testid="`smart-schedule-tab-${platform}`"
                :data-active="activePlatform === platform ? 'true' : 'false'"
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
          <p v-if="emptyPoolError" class="text-sm text-red-600" data-testid="smart-schedule-empty-error">
            {{ t('admin.users.smartSchedule.emptyPool') }}
          </p>

          <div v-if="currentDraft" class="flex min-h-0 flex-1 flex-col gap-1.5">
            <div class="flex flex-col gap-1.5" data-testid="smart-schedule-threshold-grid">
              <div
                class="rounded-lg border border-gray-200 px-2 py-1 dark:border-dark-600"
                data-testid="smart-schedule-threshold-ms-group"
              >
                <p class="mb-1 text-[11px] font-medium text-gray-700 dark:text-gray-200">
                  {{ t('admin.users.smartSchedule.thresholdMsGroup') }}
                </p>
                <div class="grid grid-cols-3 gap-x-2 gap-y-1">
                  <label class="flex min-w-0 items-center gap-1">
                    <span class="shrink-0 text-xs text-gray-500">{{ t('admin.users.smartSchedule.maxP50Ttft') }}</span>
                    <input
                      v-model.number="currentDraft.maxP50"
                      type="number"
                      min="1"
                      class="input min-w-0 flex-1 !px-2 !py-1"
                      data-testid="smart-schedule-p50"
                    />
                  </label>
                  <label class="flex min-w-0 items-center gap-1">
                    <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                      {{ t('admin.users.smartSchedule.maxP50Duration') }}
                      <HelpTooltip :content="t('admin.users.smartSchedule.maxP50DurationHint')" width-class="w-72" />
                    </span>
                    <input
                      v-model.number="currentDraft.maxP50Duration"
                      type="number"
                      min="1"
                      class="input min-w-0 flex-1 !px-2 !py-1"
                      data-testid="smart-schedule-p50-duration"
                    />
                  </label>
                  <label class="flex min-w-0 items-center gap-1">
                    <span class="shrink-0 text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityMinSuccess') }}</span>
                    <input
                      v-model.number="currentDraft.successPercent"
                      type="number"
                      min="0"
                      max="100"
                      step="0.1"
                      class="input min-w-0 flex-1 !px-2 !py-1"
                      data-testid="smart-schedule-success"
                    />
                  </label>
                </div>
              </div>

              <div
                class="grid grid-cols-1 gap-1.5 md:grid-cols-2"
                data-testid="smart-schedule-phase-groups"
              >
                <div
                  class="rounded-lg border border-gray-200 px-2 py-1 dark:border-dark-600"
                  data-testid="smart-schedule-probe-phase-group"
                >
                  <p class="mb-1 text-[11px] font-medium text-gray-700 dark:text-gray-200">
                    {{ t('admin.users.smartSchedule.probePhaseGroup') }}
                  </p>
                  <div class="grid grid-cols-2 gap-x-2 gap-y-1 sm:grid-cols-3">
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.windowNTtft') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.windowNTtftHint')" width-class="w-72" />
                      </span>
                      <input
                        :key="'window-n-ttft-' + activePlatform"
                        :value="currentDraft.windowNTtft"
                        name="smart-schedule-window-n-ttft"
                        autocomplete="off"
                        type="number"
                        min="1"
                        max="100"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-window-n-ttft"
                        @input="assignWindowN('windowNTtft', $event)"
                      />
                    </label>
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.windowNSuccess') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.windowNSuccessHint')" width-class="w-72" />
                      </span>
                      <input
                        :key="'window-n-success-' + activePlatform"
                        :value="currentDraft.windowNSuccess"
                        name="smart-schedule-window-n-success"
                        autocomplete="off"
                        type="number"
                        min="1"
                        max="100"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-window-n-success"
                        @input="assignWindowN('windowNSuccess', $event)"
                      />
                    </label>
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.maxSlowInWindow') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.maxSlowInWindowHint')" width-class="w-72" />
                      </span>
                      <input
                        v-model.number="currentDraft.maxSlowInWindow"
                        type="number"
                        min="1"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-max-slow-k"
                      />
                    </label>
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.maxConsecutiveSlow') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.maxConsecutiveSlowHint')" width-class="w-72" />
                      </span>
                      <input
                        v-model.number="currentDraft.maxConsecutiveSlow"
                        type="number"
                        min="1"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-max-slow-c"
                      />
                    </label>
                  </div>
                  <label class="mt-1 flex min-w-0 items-center gap-1">
                    <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                      {{ t('admin.users.smartSchedule.probeConcurrency') }}
                      <HelpTooltip :content="t('admin.users.smartSchedule.probeConcurrencyHint')" width-class="w-72" />
                    </span>
                    <select
                      v-model="currentDraft.probeConcurrencyMode"
                      class="input min-w-0 flex-1 !px-2 !py-1"
                      data-testid="smart-schedule-probe-concurrency-mode"
                    >
                      <option value="follow_n">{{ t('admin.users.smartSchedule.probeConcurrencyFollowN') }}</option>
                      <option value="custom">{{ t('admin.users.smartSchedule.probeConcurrencyCustom') }}</option>
                    </select>
                    <input
                      v-if="currentDraft.probeConcurrencyMode === 'custom'"
                      v-model.number="currentDraft.probeConcurrency"
                      type="number"
                      min="1"
                      max="100"
                      class="input w-16 shrink-0 !px-2 !py-1"
                      data-testid="smart-schedule-probe-concurrency"
                    />
                  </label>
                </div>

                <div
                  class="rounded-lg border border-gray-200 px-2 py-1 dark:border-dark-600"
                  data-testid="smart-schedule-sched-phase-group"
                >
                  <p class="mb-1 text-[11px] font-medium text-gray-700 dark:text-gray-200">
                    {{ t('admin.users.smartSchedule.schedPhaseGroup') }}
                  </p>
                  <div class="grid grid-cols-3 gap-x-2 gap-y-1">
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.schedWindowN') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.schedWindowNHint')" width-class="w-72" />
                      </span>
                      <input
                        v-model.number="currentDraft.schedWindowN"
                        type="number"
                        min="1"
                        max="100"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-sched-window-n"
                      />
                    </label>
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.schedMaxSlowInWindow') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.schedMaxSlowInWindowHint')" width-class="w-72" />
                      </span>
                      <input
                        v-model.number="currentDraft.schedMaxSlowInWindow"
                        type="number"
                        min="1"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-sched-max-slow-k"
                      />
                    </label>
                    <label class="flex min-w-0 items-center gap-1">
                      <span class="inline-flex shrink-0 items-center gap-0.5 text-xs text-gray-500">
                        {{ t('admin.users.smartSchedule.schedMaxConsecutiveSlow') }}
                        <HelpTooltip :content="t('admin.users.smartSchedule.schedMaxConsecutiveSlowHint')" width-class="w-72" />
                      </span>
                      <input
                        v-model.number="currentDraft.schedMaxConsecutiveSlow"
                        type="number"
                        min="1"
                        class="input min-w-0 flex-1 !px-2 !py-1"
                        data-testid="smart-schedule-sched-max-slow-c"
                      />
                    </label>
                  </div>
                </div>
              </div>

              <div
                class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-gray-200 px-2 py-1 dark:border-dark-600"
                data-testid="smart-schedule-cooldown-row"
              >
                <label class="flex min-w-0 items-center gap-1">
                  <span class="shrink-0 text-xs text-gray-500">{{ t('admin.users.smartSchedule.cooldown') }}</span>
                  <input
                    v-model.number="currentDraft.cooldownMinutes"
                    type="number"
                    min="1"
                    max="1440"
                    class="input w-20 !px-2 !py-1"
                    data-testid="smart-schedule-cooldown"
                  />
                </label>
                <div class="flex min-w-0 items-center gap-1">
                  <div
                    class="inline-flex overflow-hidden rounded-md border border-gray-200 dark:border-dark-600"
                    data-testid="smart-schedule-soft-cooldown"
                  >
                    <button
                      type="button"
                      class="px-2 py-0.5 text-xs"
                      :class="!currentDraft.softCooldown ? 'bg-gray-800 text-white dark:bg-gray-200 dark:text-gray-900' : 'bg-white text-gray-600 dark:bg-dark-800 dark:text-gray-300'"
                      :aria-pressed="!currentDraft.softCooldown"
                      @click="currentDraft.softCooldown = false"
                    >
                      {{ t('admin.users.smartSchedule.cooldownModeHard') }}
                    </button>
                    <button
                      type="button"
                      class="px-2 py-0.5 text-xs"
                      :class="currentDraft.softCooldown ? 'bg-gray-800 text-white dark:bg-gray-200 dark:text-gray-900' : 'bg-white text-gray-600 dark:bg-dark-800 dark:text-gray-300'"
                      :aria-pressed="currentDraft.softCooldown"
                      @click="currentDraft.softCooldown = true"
                    >
                      {{ t('admin.users.smartSchedule.cooldownModeSoft') }}
                    </button>
                  </div>
                  <HelpTooltip
                    :content="currentDraft.softCooldown ? t('admin.users.smartSchedule.cooldownModeSoftHint') : t('admin.users.smartSchedule.cooldownModeHardHint')"
                    width-class="w-72"
                  />
                </div>
                <label class="flex min-w-0 items-center gap-1">
                  <span class="shrink-0 text-xs text-gray-500">{{ t('admin.accounts.userSchedule.qualityCondition') }}</span>
                  <select v-model="currentDraft.condition" class="input w-24 !px-2 !py-1">
                    <option value="or">{{ t('admin.accounts.userSchedule.qualityConditionOr') }}</option>
                    <option value="and">{{ t('admin.accounts.userSchedule.qualityConditionAnd') }}</option>
                  </select>
                </label>
              </div>
            </div>

            <div
              class="flex items-start justify-between gap-3 rounded-lg border border-gray-200 px-2 py-1.5 dark:border-dark-600"
              data-testid="smart-schedule-failover-toggle"
            >
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-800 dark:text-gray-100">
                  {{ t('admin.accounts.stability.failoverToggle') }}
                </p>
                <p class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.qualityHardClose.scheduleUseFailoverHint') }}
                </p>
              </div>
              <Toggle :model-value="scheduleUseFailover" @update:model-value="saveFailoverToggle" />
            </div>

            <div class="flex flex-wrap items-center gap-1.5">
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
              <select v-model="copyFromPlatform" class="input min-w-[8rem] flex-1 !px-2 !py-1" data-testid="smart-schedule-copy-from">
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
                :class="isDirty ? 'ring-2 ring-amber-400' : ''"
                :disabled="submitting || loading"
                data-testid="smart-schedule-save"
                @click="onSave"
              >
                {{ submitting ? t('admin.users.smartSchedule.saving') : t('admin.users.smartSchedule.save') }}
              </button>
              <span v-if="isDirty" class="text-xs text-amber-600 dark:text-amber-400" data-testid="smart-schedule-dirty-hint">
                {{ t('admin.users.smartSchedule.dirtySaveHint') }}
              </span>
            </div>
          </div>
        </aside>

        <section
          class="flex h-full min-w-0 flex-col gap-1.5 rounded-xl border border-gray-200 bg-white px-2 py-1.5 dark:border-dark-700 dark:bg-dark-800"
          data-testid="smart-schedule-pool-panel"
        >
          <div class="flex flex-wrap items-center gap-2">
            <h2
              class="text-sm font-semibold text-gray-900 dark:text-white"
              :title="t('admin.users.smartSchedule.poolHint')"
            >
              {{ t('admin.users.smartSchedule.poolTitle', { platform: platformLabel(activePlatform) }) }}
            </h2>
          </div>

          <SmartSchedulePoolAddBar
            class="min-w-0"
            v-model:search-query="accountSearchQuery"
            v-model:search-open="accountSearchOpen"
            :filtered-accounts="filteredAddableAccounts"
            :api-count="addableSchedulingApi.length"
            :oauth-count="addableSchedulingOauth.length"
            :all-count="addableSchedulingAll.length"
            :candidates-ready="candidatesReady"
            :pool-empty="poolTableRows.length === 0"
            :auto-sorting="autoSorting"
            :auto-sort-done="autoSortDone"
            :auto-sort-total="autoSortTotal"
            :refreshing="refreshing"
            :refresh-disabled="refreshing"
            :auto-refresh-enabled="autoRefreshEnabled"
            :auto-refresh-countdown="autoRefreshCountdown"
            :auto-refresh-interval-seconds="autoRefreshIntervalSeconds"
            :auto-refresh-intervals="autoRefreshIntervals"
            :auto-sort-enabled="autoSortEnabled"
            @search-blur="onAccountSearchBlur"
            @choose="chooseAddableAccount"
            @open-filtered-add="openFilteredAdd"
            @add-scheduling="addSchedulingAccounts"
            @auto-sort="handlePoolAutoSort"
            @refresh="handleManualRefresh"
            @set-auto-refresh-enabled="setAutoRefreshEnabled"
            @set-auto-refresh-interval="setAutoRefreshInterval"
            @set-auto-sort-enabled="setAutoSortEnabled"
          />

          <SmartSchedulePoolFilters class="min-w-0" v-model:filters="poolFilters" />
        </section>
        </div>

        <div class="space-y-2 overflow-visible" data-testid="smart-schedule-pool-table-region">
          <p
            v-if="isDirty"
            class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-1.5 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
            data-testid="smart-schedule-dirty-banner"
          >
            {{ t('admin.users.smartSchedule.dirtyBanner') }}
          </p>

          <div class="flex flex-wrap items-start justify-between gap-2">
            <SmartSchedulePoolBulkBar
              v-if="poolTableRows.length > 0"
              class="min-w-0 flex-1"
              :selected-ids="filteredSelectedIds"
              :filtered-count="filteredPoolRows.length"
              :bulk-cap="bulkCap"
              @update:bulk-cap="bulkCap = $event"
              @select-page="selectMatching(filteredPoolRows.map((row) => row.id))"
              @select-matching="selectMatching(filteredPoolRows.map((row) => row.id))"
              @clear="clearSelection"
              @apply-cap="applyCapToAccounts(filteredSelectedIds)"
              @apply-cap-all="applyCapToAll"
              @remove="removeAccounts(filteredSelectedIds)"
            />
            <div class="relative ml-auto shrink-0" ref="columnDropdownRef">
              <button
                type="button"
                class="btn btn-secondary px-2 py-1 md:px-3"
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
                class="absolute right-0 z-50 mt-2 w-72 origin-top-right rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
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

          <div class="smart-schedule-pool-table-scroll overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
            <DataTable
              ref="poolTableRef"
              :columns="poolColumns"
              :data="filteredPoolRows"
              :loading="false"
              row-key="id"
              :virtual-scroll="true"
              :estimate-row-height="104"
              :resizable-columns="true"
              default-sort-key="sort_order"
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
              <template #cell-select="{ row }">
                <div class="flex items-center gap-1.5">
                  <input
                    type="checkbox"
                    :checked="selectedAccountIds.includes(row.id)"
                    class="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :data-testid="`smart-schedule-select-${row.id}`"
                    @change="toggleAccountSelection(row.id)"
                  />
                  <button
                    type="button"
                    class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-primary-600 text-white shadow-sm transition-colors hover:bg-primary-700"
                    :title="t('admin.accounts.moveToTop')"
                    :aria-label="t('admin.accounts.moveToTop')"
                    data-testid="account-move-to-top"
                    @click.stop="handleMoveToTop(row)"
                  >
                    <Icon name="arrowUp" size="sm" :stroke-width="2.5" class="text-white" />
                  </button>
                </div>
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
              <template #cell-claude_gpt_bridge="{ row }">
                <span class="text-xs text-gray-600 dark:text-gray-300">
                  {{
                    row.platform === 'openai'
                      ? row.extra?.openai_claude_gpt_bridge_enabled
                        ? t('admin.users.smartSchedule.claudeGptBridgeOn')
                        : t('admin.users.smartSchedule.claudeGptBridgeOff')
                      : '—'
                  }}
                </span>
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
                    :color-class="pairCapacityClass(memberCurrent(row.id), pairBadgeMax(row.id))"
                    :current="memberCurrent(row.id)"
                    :max="pairBadgeMax(row.id)"
                    :tooltip="pairBadgeTooltip(row.id)"
                    data-testid="smart-schedule-pair-badge"
                  />
                </div>
              </template>
              <template #header-admission="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.admissionHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-admission="{ row }">
                <div
                  class="flex min-w-0 flex-col gap-0.5"
                  data-testid="smart-schedule-admission"
                  :data-admission="row.admission"
                  :title="coolingCellTitle(row)"
                >
                  <div class="flex min-w-0 flex-wrap items-center gap-x-1 gap-y-0.5">
                    <span
                      class="inline-flex w-fit shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium"
                      :class="admissionChipClass(row.admission)"
                    >
                      {{ admissionLabel(row.admission) }}
                    </span>
                    <template v-if="row.admission === 'cooling'">
                      <span
                        v-if="row.soft_cooldown"
                        class="shrink-0 text-[10px] text-amber-700 dark:text-amber-300"
                        data-testid="smart-schedule-soft-chip"
                      >
                        {{ t('admin.users.smartSchedule.admissionSoft') }}
                      </span>
                      <span
                        v-if="row.cooldown_until"
                        class="min-w-0 truncate text-[10px] text-amber-700 dark:text-amber-300"
                      >
                        {{ t('admin.users.smartSchedule.admissionCoolingRemaining', { minutes: cooldownRemainingMinutes(row.cooldown_until) }) }}
                      </span>
                      <span
                        v-if="row.soft_cooldown && row.soft_cooldown_progress"
                        class="min-w-0 truncate text-[10px] text-amber-700 dark:text-amber-300"
                        data-testid="smart-schedule-soft-progress"
                      >
                        {{ formatSoftCooldownProgress(row.soft_cooldown_progress) }}
                      </span>
                    </template>
                  </div>
                  <span
                    v-if="row.admission === 'cooling' && row.cooldown_reason"
                    class="min-w-0 truncate text-[10px] text-gray-500 dark:text-gray-400"
                    :title="row.cooldown_reason"
                    data-testid="smart-schedule-cooldown-reason"
                  >
                    {{ row.cooldown_reason }}
                  </span>
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
                  :window-n="accountQualityWindowN"
                  :loading="statsLoading"
                  @click="openStabilityDialog(row)"
                />
              </template>
              <template #header-pair_quality="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.pairQualityHint')" width-class="w-80" />
                </div>
              </template>
              <template #cell-pair_quality="{ row }">
                <SmartSchedulePairQualityCell
                  :quality="pairQualityById[String(row.id)] ?? null"
                  :loading="statsLoading"
                  @click="openPairQualityDialog(row)"
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
              <template #header-schedule_pnl="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.poolPnlHint')" width-class="w-72" />
                </div>
              </template>
              <template #cell-schedule_pnl="{ row }">
                <SmartSchedulePnlCell
                  :account="row"
                  :summary="pairPnlById[String(row.id)] ?? null"
                  :today-stats="todayStatsById[String(row.id)] ?? null"
                  :loading="statsLoading"
                  :balance-refreshing="isBalanceRefreshing(row.id)"
                  @click="openPairSchedulePnl(row)"
                  @refresh-balance="void refreshAccountBalance(row.id)"
                />
              </template>
              <template #header-sort_order="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.poolSortOrderHint')" width-class="w-64" />
                </div>
              </template>
              <template #cell-sort_order="{ row }">
                <span
                  class="text-sm tabular-nums text-gray-700 dark:text-gray-200"
                  :data-testid="`smart-schedule-pool-sort-order-${row.id}`"
                >
                  {{ row.sort_order ?? '—' }}
                </span>
              </template>
              <template #header-priority="{ column }">
                <div class="flex items-center">
                  <span>{{ column.label }}</span>
                  <HelpTooltip :content="t('admin.users.smartSchedule.accountPriorityHint')" width-class="w-64" />
                </div>
              </template>
              <template #cell-priority="{ row }">
                <div class="contents" :data-testid="`smart-schedule-pool-priority-${row.id}`">
                  <AccountInlineNumberCell
                    :model-value="liveAccountPriority(row)"
                    :min="0"
                    :disabled="inlineSavingId === row.id"
                    :hint="t('admin.users.smartSchedule.accountPriorityHint')"
                    @save="(value) => handleInlinePriority(row, value)"
                  />
                </div>
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
                    data-testid="account-edit"
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
                  <SmartScheduleAdmissionSwitch
                    :admission="row.admission"
                    :paused="row.paused"
                    :pinned="row.pinned"
                    :disabled="row.admission === 'unsaved_preview'"
                    @select="setPairAdmission(row.id, $event)"
                  />
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
        </div>
      </template>
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
    <SmartSchedulePairQualityDialog
      :show="pairQualityAcc != null"
      :user-id="userId"
      :account="pairQualityAcc"
      :platform="activePlatform"
      @close="pairQualityAcc = null"
    />
    <UserQualityDialog
      :show="userQualityDialogOpen"
      :user-id="userQualityDialogOpen ? userId : null"
      :title="user?.email || user?.username || String(userId)"
      @close="userQualityDialogOpen = false"
    />
    <SchedulePnlTrendDialog
      :show="schedulePnlDialog != null"
      :user-id="userId"
      :account-id="schedulePnlDialog?.accountId ?? null"
      :account="schedulePnlDialog?.account ?? null"
      :title="schedulePnlDialog?.title"
      @close="schedulePnlDialog = null"
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
    <SmartScheduleAddAccountDialog
      :show="showAddDialog"
      :platform="activePlatform"
      :platform-label="platformLabel(activePlatform)"
      :accounts="addableAccounts"
      :groups="groups"
      :proxies="proxies"
      @close="showAddDialog = false"
      @add="onFilteredAdd"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AdminUser } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type { SchedulePnlSummary } from '@/api/admin/users'
import type { BatchUserBurnRateStats, BatchUserUsageStats } from '@/api/admin/dashboard'
import type { Column } from '@/components/common/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import {
  cooldownRemainingMinutes,
  EMPTY_SMART_SCHEDULE_POOL_FILTERS,
  matchesPoolFilters,
  pairOccupancyDisplayMaxForAdmission,
  resolvePoolAdmission,
  resolveQualityAdmissionHint,
  type PoolAdmissionState,
  type SmartSchedulePoolFilters as PoolFilterState
} from '@/composables/smartSchedulePoolAdmission'
import {
  assignPoolAutoSortOrders,
  assignPoolMoveToTopSortOrders,
  effectivePoolAutoSortRate,
  poolSortOrdersUnchanged,
  sortSmartSchedulePoolMembers
} from '@/composables/smartSchedulePoolAutoSort'
import { pickBatchUserStat, smartScheduleSummaryFromDrafts } from '@/composables/adminUserListRow'
import { useAppStore } from '@/stores/app'
import { useUserSmartScheduleEditor } from '@/composables/useUserSmartScheduleEditor'
import SmartScheduleAdmissionSwitch from '@/components/admin/smart-schedule/SmartScheduleAdmissionSwitch.vue'
import { useSmartSchedulePoolAccountOps } from '@/composables/useSmartSchedulePoolAccountOps'
import {
  readSmartSchedulePoolFetchNeeds,
  useSmartSchedulePoolColumnLayout
} from '@/composables/useSmartSchedulePoolColumnLayout'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DataTable from '@/components/common/DataTable.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountQualityCell from '@/components/account/AccountQualityCell.vue'
import { ACCOUNT_QUALITY_WINDOW_N_DEFAULT, resolveAccountQualityWindowN } from '@/utils/accountQualityWindowN'
import SmartSchedulePairQualityCell from '@/components/admin/smart-schedule/SmartSchedulePairQualityCell.vue'
import SmartSchedulePairQualityDialog from '@/components/admin/smart-schedule/SmartSchedulePairQualityDialog.vue'
import AccountTodayStatsCell from '@/components/account/AccountTodayStatsCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import SmartSchedulePnlCell from '@/components/admin/user/SmartSchedulePnlCell.vue'
import SchedulePnlTrendDialog from '@/components/admin/user/SchedulePnlTrendDialog.vue'
import UserQualityDialog from '@/components/admin/user/UserQualityDialog.vue'
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
import SmartSchedulePoolFilters from '@/components/admin/smart-schedule/SmartSchedulePoolFilters.vue'
import SmartSchedulePoolBulkBar from '@/components/admin/smart-schedule/SmartSchedulePoolBulkBar.vue'
import SmartSchedulePoolAddBar from '@/components/admin/smart-schedule/SmartSchedulePoolAddBar.vue'
import SmartScheduleAddAccountDialog from '@/components/admin/smart-schedule/SmartScheduleAddAccountDialog.vue'
import AdminUserListRowTable from '@/components/admin/user/AdminUserListRowTable.vue'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const user = ref<AdminUser | null>(null)
const scheduleUseFailover = ref(false)
const accountQualityWindowN = ref(ACCOUNT_QUALITY_WINDOW_N_DEFAULT)
const userLoading = ref(false)
const userQualityStats = ref<AccountQualityStats | null>(null)
const userQualityLoading = ref(false)
const userQualityError = ref<string | null>(null)
const userUsageStats = ref<BatchUserUsageStats | null>(null)
const userBurnRateStats = ref<BatchUserBurnRateStats | null>(null)
const userSchedulePnl = ref<SchedulePnlSummary | null>(null)
const userSchedulePnlLoading = ref(false)
const schedulePnlDialog = ref<{ accountId?: number; account?: Account; title: string } | null>(null)
const userQualityDialogOpen = ref(false)
const pairQualityAcc = ref<Account | null>(null)
const accountSearchQuery = ref('')
const accountSearchOpen = ref(false)
const showAddDialog = ref(false)
const poolFilters = ref<PoolFilterState>({ ...EMPTY_SMART_SCHEDULE_POOL_FILTERS })
try {
  const raw = localStorage.getItem('smart-schedule-pool-sort')
  if (raw) {
    const parsed = JSON.parse(raw) as { key?: string; order?: 'asc' | 'desc' }
    if (parsed?.key === 'priority') {
      localStorage.setItem('smart-schedule-pool-sort', JSON.stringify({ key: 'sort_order', order: 'asc' }))
    }
  }
} catch {
  // ignore private-mode / invalid JSON
}

const AUTO_REFRESH_STORAGE_KEY = 'smart-schedule-auto-refresh'
const AUTO_SORT_STORAGE_KEY = 'smart-schedule-auto-sort'
const autoRefreshIntervals = [5, 10, 15, 30] as const
const autoRefreshEnabled = ref(false)
const autoRefreshIntervalSeconds = ref<(typeof autoRefreshIntervals)[number]>(5)
const autoRefreshCountdown = ref(0)
const autoSortEnabled = ref(false)
const poolTableRef = ref<{ setSort?: (key: string, order: 'asc' | 'desc') => void } | null>(null)
const autoSorting = ref(false)
const autoSortDone = ref(0)
const autoSortTotal = ref(0)
let autoRefreshTimer: number | null = null

const userId = computed(() => {
  const raw = route.params.id
  const value = Number(Array.isArray(raw) ? raw[0] : raw)
  return Number.isFinite(value) && value > 0 ? value : null
})

const poolFetchNeeds = reactive(readSmartSchedulePoolFetchNeeds())

const {
  platforms,
  loading,
  initialLoaded,
  submitting,
  copying,
  statsLoading,
  refreshing,
  candidatesReady,
  emptyPoolError,
  isDirty,
  activePlatform,
  copyFromPlatform,
  bulkCap,
  drafts,
  poolAccounts,
  qualityStatsById,
  todayStatsById,
  pairPnlById,
  pairQualityById,
  currentDraft,
  currentSavedDraft,
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
  memberCapOrNull,
  memberCurrent,
  memberCooldownUntil,
  memberCooldownReason,
  memberSoftCooldownProgress,
  memberPaused,
  memberProbing,
  memberPinned,
  memberProbeCap,
  memberSortOrder,
  persistSortOrders,
  memberResumeActive,
  memberResumeChipActive,
  setMemberCap,
  patchPoolAccount,
  applyCapToAll,
  applyCapToAccounts,
  selectedAccountIds,
  toggleAccountSelection,
  selectMatching,
  clearSelection,
  addAccountById,
  addAccountsByIds,
  addSchedulingAccounts,
  removeAccount,
  removeAccounts,
  onToggleEnabled,
  onSave,
  onCopy,
  setPairAdmission,
  refreshAll,
  ensureCandidates,
  refreshAccountBalance,
  isBalanceRefreshing
} = useUserSmartScheduleEditor(userId, { poolFetchNeeds })

function assignWindowN(field: 'windowNTtft' | 'windowNSuccess', event: Event) {
  const draft = currentDraft.value
  if (!draft) return
  const text = (event.target as HTMLInputElement).value
  if (text.trim() === '') {
    draft[field] = ''
    return
  }
  const n = Number(text)
  if (!Number.isFinite(n)) return
  draft[field] = n
}

const pageReady = computed(() => initialLoaded.value && user.value != null)

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
} = useSmartSchedulePoolAccountOps({
  patchPoolAccount
})

const allPoolColumns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false, minWidth: 88, resizable: false },
  { key: 'name', label: t('admin.accounts.columns.name'), sortable: true, minWidth: 140 },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType'), sortable: true, minWidth: 120 },
  { key: 'claude_gpt_bridge', label: t('admin.users.smartSchedule.claudeGptBridge'), sortable: true, minWidth: 110 },
  { key: 'concurrency', label: t('admin.accounts.columns.capacity'), sortable: true, minWidth: 88 },
  { key: 'pair_cap', label: t('admin.users.smartSchedule.pairCap'), sortable: true, minWidth: 88 },
  { key: 'admission', label: t('admin.users.smartSchedule.admission'), sortable: true, minWidth: 180 },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: true, minWidth: 88 },
  { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: true, minWidth: 88 },
  { key: 'quality_ttft', label: t('admin.accounts.columns.quality'), sortable: false, minWidth: 88 },
  { key: 'pair_quality', label: t('admin.users.smartSchedule.pairQuality'), sortable: false, minWidth: 100 },
  { key: 'today_stats', label: t('admin.accounts.columns.todayStats'), sortable: false, minWidth: 88 },
  { key: 'groups', label: t('admin.accounts.columns.groups'), sortable: false, minWidth: 88 },
  { key: 'schedule_pnl', label: t('admin.accounts.columns.schedulePnl'), sortable: false, minWidth: 160 },
  { key: 'sort_order', label: t('admin.users.smartSchedule.poolSortOrder'), sortable: true, minWidth: 72 },
  { key: 'priority', label: t('admin.users.smartSchedule.accountPriority'), sortable: true, minWidth: 72 },
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

watch(
  () => [isColumnVisible('quality_ttft'), isColumnVisible('today_stats'), isColumnVisible('schedule_pnl')] as const,
  ([quality, today, pnl]) => {
    poolFetchNeeds.quality = quality
    poolFetchNeeds.today = today || pnl
    poolFetchNeeds.pnl = pnl
  },
  { immediate: true }
)

const poolTableRows = computed(() =>
  poolAccounts.value.map((account) => {
    const admission = resolvePoolAdmission({
      account,
      pairCap: memberCapOrNull(account.id),
      pairCurrent: memberCurrent(account.id),
      cooldownUntil: memberCooldownUntil(account.id),
      paused: memberPaused(account.id),
      probing: memberProbing(account.id),
      pinned: memberPinned(account.id),
      qualityHint: resolveQualityAdmissionHint({
        draft: currentDraft.value,
        saved: currentSavedDraft.value,
        pairQuality: pairQualityById.value[String(account.id)],
        resumeActive: memberResumeActive(account.id),
        resumeChipActive: memberResumeChipActive(account.id)
      })
    })
    return {
      ...account,
      platform_type: `${account.platform} ${account.type}`,
      pair_cap: memberCap(account.id),
      pair_current: memberCurrent(account.id),
      cooldown_until: memberCooldownUntil(account.id),
      cooldown_reason: memberCooldownReason(account.id),
      soft_cooldown: Boolean(currentDraft.value?.softCooldown),
      soft_cooldown_progress: memberSoftCooldownProgress(account.id),
      sort_order: memberSortOrder(account.id),
      priority: liveAccountPriority(account),
      paused: memberPaused(account.id),
      pinned: memberPinned(account.id),
      admission: admission.state
    }
  })
)

const filteredPoolRows = computed(() =>
  poolTableRows.value.filter((row) => matchesPoolFilters(row, row.admission, poolFilters.value))
)

const filteredSelectedIds = computed(() => {
  const visible = new Set(filteredPoolRows.value.map((row) => row.id))
  return selectedAccountIds.value.filter((id) => visible.has(id))
})

const userSmartScheduleSummary = computed(() =>
  smartScheduleSummaryFromDrafts(platforms, drafts)
)

function admissionLabel(state: PoolAdmissionState) {
  switch (state) {
    case 'paused':
      return t('admin.users.smartSchedule.admissionPaused')
    case 'cooling':
      return t('admin.users.smartSchedule.admissionCooling')
    case 'pair_full':
      return t('admin.users.smartSchedule.admissionPairFull')
    case 'stopped':
      return t('admin.users.smartSchedule.admissionStopped')
    case 'will_cool':
      return t('admin.users.smartSchedule.admissionWillCool')
    case 'unsaved_preview':
      return t('admin.users.smartSchedule.admissionUnsavedPreview')
    case 'resumed':
      return t('admin.users.smartSchedule.admissionResumed')
    case 'pinned':
      return t('admin.users.smartSchedule.admissionPinned')
    case 'probing':
      return t('admin.users.smartSchedule.admissionProbing')
    default:
      return t('admin.users.smartSchedule.admissionSelectable')
  }
}

function coolingCellTitle(row: { admission: PoolAdmissionState; cooldown_until?: string | null }) {
  if (row.admission === 'cooling' && row.cooldown_until) {
    return t('admin.users.smartSchedule.admissionCoolingUntil', { time: formatDateTime(row.cooldown_until) })
  }
  return admissionTitle(row.admission)
}

function formatSoftCooldownProgress(progress: {
  ttft_count: number
  n_ttft: number
  ok_count: number
  n_ok: number
  duration_count?: number
  n_duration?: number
}) {
  const parts = [`${progress.ttft_count}/${progress.n_ttft}`, `${progress.ok_count}/${progress.n_ok}`]
  if (progress.n_duration && progress.n_duration > 0) {
    parts.push(`${progress.duration_count ?? 0}/${progress.n_duration}`)
  }
  return parts.join(' · ')
}

function admissionTitle(state: PoolAdmissionState) {
  switch (state) {
    case 'will_cool':
      return t('admin.users.smartSchedule.admissionWillCoolHint')
    case 'unsaved_preview':
      return t('admin.users.smartSchedule.admissionUnsavedPreviewHint')
    case 'resumed':
      return t('admin.users.smartSchedule.admissionResumedHint')
    case 'pinned':
      return t('admin.users.smartSchedule.admissionPinnedHint')
    case 'paused':
      return t('admin.users.smartSchedule.admissionPausedHint')
    case 'probing':
      return t('admin.users.smartSchedule.admissionProbingHint')
    default:
      return ''
  }
}

function admissionChipClass(state: PoolAdmissionState) {
  switch (state) {
    case 'paused':
      return 'bg-slate-200 text-slate-800 dark:bg-slate-800/70 dark:text-slate-200'
    case 'cooling':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
    case 'will_cool':
      return 'bg-orange-50 text-orange-800 dark:bg-orange-900/30 dark:text-orange-200'
    case 'unsaved_preview':
      return 'bg-slate-100 text-slate-700 dark:bg-dark-600 dark:text-slate-300'
    case 'pair_full':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'stopped':
      return 'bg-gray-200 text-gray-700 dark:bg-dark-600 dark:text-gray-300'
    case 'resumed':
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-300'
    case 'pinned':
      return 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/40 dark:text-indigo-300'
    case 'probing':
      return 'bg-violet-100 text-violet-800 dark:bg-violet-900/40 dark:text-violet-300'
    default:
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  }
}

function getAccountEmail(row: Account): string | undefined {
  const email = row.extra?.email_address || row.credentials?.email || row.parent_email
  return typeof email === 'string' ? email : undefined
}

function liveAccountPriority(account: { id: number; priority?: number }): number {
  const live = poolAccounts.value.find((item) => item.id === account.id)
  const value = live?.priority ?? account.priority
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function pairBadgeMax(accountId: number) {
  return pairOccupancyDisplayMaxForAdmission({
    probing: memberProbing(accountId),
    pinned: memberPinned(accountId),
    pairCap: memberCapOrNull(accountId),
    windowNSuccess: currentDraft.value?.windowNSuccess,
    windowN: currentDraft.value?.windowNSuccess,
    backendCap: memberProbeCap(accountId),
    mode: currentDraft.value?.probeConcurrencyMode,
    probeConcurrency: currentDraft.value?.probeConcurrency
  })
}

function pairBadgeTooltip(accountId: number) {
  if (memberProbing(accountId) && !memberPinned(accountId)) {
    return t('admin.users.smartSchedule.pairOccupancyProbingHint')
  }
  return memberCapOrNull(accountId) == null
    ? t('admin.users.smartSchedule.pairOccupancyUncappedHint')
    : t('admin.users.smartSchedule.pairCapHint')
}

function pairCapacityClass(current: number, max: number) {
  if (max > 0 && current >= max) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (current > 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
}

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

async function openFilteredAdd() {
  await ensureCandidates()
  showAddDialog.value = true
}

watch(accountSearchOpen, (open) => {
  if (open) void ensureCandidates()
})

async function onFilteredAdd(accountIds: number[]) {
  const added = await addAccountsByIds(accountIds)
  showAddDialog.value = false
  if (added > 0) {
    appStore.showSuccess(t('admin.users.smartSchedule.addFilteredSuccess', { count: String(added) }))
  }
}

function onAccountSearchBlur() {
  window.setTimeout(() => {
    accountSearchOpen.value = false
  }, 120)
}

function platformLabel(platform: string) {
  return t(`admin.groups.platforms.${platform}`)
}

function showPoolSortOrderSort() {
  poolTableRef.value?.setSort?.('sort_order', 'asc')
}

function currentPoolSortStates() {
  return poolTableRows.value.map((row) => ({
    id: row.id,
    sortOrder: memberSortOrder(row.id)
  }))
}

async function persistAssignedPoolOrder(
  assigned: Array<{ id: number; sortOrder: number }>,
  errorKey?: string,
  options?: { silent?: boolean }
) {
  return persistSortOrders(
    assigned.map((row) => ({ account_id: row.id, sort_order: row.sortOrder })),
    errorKey,
    options
  )
}

async function handleMoveToTop(account: Account) {
  if (autoSorting.value) return
  const assigned = assignPoolMoveToTopSortOrders(currentPoolSortStates(), account.id)
  if (assigned.length === 0) return
  if (poolSortOrdersUnchanged(currentPoolSortStates(), assigned)) {
    showPoolSortOrderSort()
    appStore.showSuccess(t('admin.accounts.moveToTopSuccess'))
    return
  }
  autoSorting.value = true
  autoSortDone.value = 0
  autoSortTotal.value = 1
  try {
    const ok = await persistAssignedPoolOrder(assigned, 'admin.accounts.moveToTopFailed')
    if (!ok) return
    showPoolSortOrderSort()
    appStore.showSuccess(t('admin.accounts.moveToTopSuccess'))
  } finally {
    autoSorting.value = false
  }
}

async function handlePoolAutoSort(options?: { silent?: boolean }) {
  if (autoSorting.value || poolTableRows.value.length === 0) return
  if (options?.silent && (refreshing.value || loading.value)) return
  const sorted = sortSmartSchedulePoolMembers(
    poolTableRows.value.map((row) => ({
      id: row.id,
      admission: row.admission,
      pairCap: memberCapOrNull(row.id),
      pairCurrent: row.pair_current ?? 0,
      concurrency: row.concurrency ?? 1,
      priority: row.priority ?? 0,
      upstreamRate: effectivePoolAutoSortRate(row.upstream_rate_multiplier, row.type)
    }))
  )
  const assigned = assignPoolAutoSortOrders(sorted)
  if (poolSortOrdersUnchanged(currentPoolSortStates(), assigned)) {
    showPoolSortOrderSort()
    if (!options?.silent) {
      appStore.showInfo(t('admin.users.smartSchedule.autoSortUnchanged'))
    }
    return
  }
  autoSorting.value = true
  autoSortDone.value = 0
  autoSortTotal.value = 1
  try {
    const ok = await persistAssignedPoolOrder(assigned, undefined, { silent: options?.silent })
    if (!ok) return
    showPoolSortOrderSort()
    if (!options?.silent) {
      appStore.showSuccess(t('admin.users.smartSchedule.autoSortSuccess', { count: assigned.length }))
    }
  } finally {
    autoSorting.value = false
  }
}

function goBack() {
  void router.push({ name: 'AdminUsers' })
}

function openUserSchedulePnl() {
  schedulePnlDialog.value = { title: t('admin.users.schedulePnl.dialogTitle') }
}

function openUserQuality() {
  userQualityDialogOpen.value = true
}

function openPairSchedulePnl(account: Account) {
  schedulePnlDialog.value = {
    accountId: account.id,
    account,
    title: t('admin.users.schedulePnl.dialogTitlePair', { account: account.name || String(account.id) })
  }
}

function openPairQualityDialog(account: Account) {
  pairQualityAcc.value = account
}

async function loadUserListRowExtras(detail: AdminUser) {
  const id = detail.id
  userQualityLoading.value = true
  userQualityError.value = null
  // Visible header extras only. burn_rate is shown (UsersView POST /admin/dashboard/users-burn-rate).
  // username is not a header column — do not add username-only fetches.
  const tasks: Promise<void>[] = [
    (async () => {
      try {
        const qualityResponse = await adminAPI.users.getBatchQualityStats([id])
        if (user.value?.id !== id) return
        userQualityStats.value = pickBatchUserStat(qualityResponse.stats, id)
      } catch {
        if (user.value?.id !== id) return
        userQualityError.value = 'Failed'
        userQualityStats.value = null
      } finally {
        if (user.value?.id === id) userQualityLoading.value = false
      }
    })(),
    (async () => {
      try {
        const usageResponse = await adminAPI.dashboard.getBatchUsersUsage([id])
        if (user.value?.id !== id) return
        userUsageStats.value = pickBatchUserStat(usageResponse.stats, id)
      } catch {
        if (user.value?.id !== id) return
        userUsageStats.value = null
      }
    })(),
    (async () => {
      userSchedulePnlLoading.value = true
      try {
        const pnlResponse = await adminAPI.users.getBatchSmartSchedulePnlSummaries([id])
        if (user.value?.id !== id) return
        userSchedulePnl.value = pickBatchUserStat(pnlResponse.summaries, id)
      } catch {
        if (user.value?.id !== id) return
        userSchedulePnl.value = null
      } finally {
        if (user.value?.id === id) userSchedulePnlLoading.value = false
      }
    })(),
    (async () => {
      try {
        const burnRateResponse = await adminAPI.dashboard.getBatchUsersBurnRate([id])
        if (user.value?.id !== id) return
        userBurnRateStats.value = pickBatchUserStat(burnRateResponse.stats, id)
      } catch {
        if (user.value?.id !== id) return
        userBurnRateStats.value = null
      }
    })(),
    (async () => {
      try {
        const listed = await adminAPI.users.list(1, 5, {
          search: detail.email,
          include_subscriptions: true
        })
        if (user.value?.id !== id) return
        const match = listed.items.find((item) => item.id === id)
        if (!match) return
        user.value = {
          ...user.value,
          current_concurrency: match.current_concurrency,
          subscriptions: match.subscriptions ?? user.value.subscriptions,
          last_used_at: match.last_used_at ?? user.value.last_used_at,
          last_active_at: match.last_active_at ?? user.value.last_active_at
        }
      } catch {
        // getById already has the core user DTO; list-only fields stay empty
      }
    })()
  ]
  await Promise.allSettled(tasks)
}

async function loadUser(options?: { silent?: boolean }) {
  if (!userId.value) return
  if (!options?.silent) userLoading.value = true
  try {
    const detail = await adminAPI.users.getById(userId.value)
    user.value = detail
    void loadUserListRowExtras(detail)
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.users.smartSchedule.loadFailed')))
  } finally {
    if (!options?.silent) userLoading.value = false
  }
}

function loadSavedAutoRefresh() {
  try {
    const saved = localStorage.getItem(AUTO_REFRESH_STORAGE_KEY)
    if (!saved) return
    const parsed = JSON.parse(saved) as { enabled?: boolean; interval_seconds?: number }
    autoRefreshEnabled.value = parsed.enabled === true
    const interval = Number(parsed.interval_seconds)
    if (autoRefreshIntervals.includes(interval as (typeof autoRefreshIntervals)[number])) {
      autoRefreshIntervalSeconds.value = interval as (typeof autoRefreshIntervals)[number]
    }
  } catch {
    // ignore
  }
}

function saveAutoRefreshToStorage() {
  try {
    localStorage.setItem(
      AUTO_REFRESH_STORAGE_KEY,
      JSON.stringify({
        enabled: autoRefreshEnabled.value,
        interval_seconds: autoRefreshIntervalSeconds.value
      })
    )
  } catch {
    // ignore
  }
}

function stopAutoRefreshTimer() {
  if (autoRefreshTimer != null) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

function startAutoRefreshTimer() {
  stopAutoRefreshTimer()
  if (!autoRefreshEnabled.value) return
  autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
  autoRefreshTimer = window.setInterval(() => {
    if (!autoRefreshEnabled.value) return
    if (loading.value || refreshing.value || autoSorting.value) return
    if (autoRefreshCountdown.value <= 1) {
      autoRefreshCountdown.value = autoRefreshIntervalSeconds.value
      void handleAutoRefresh()
      return
    }
    autoRefreshCountdown.value -= 1
  }, 1000)
}

function setAutoRefreshEnabled(enabled: boolean) {
  autoRefreshEnabled.value = enabled
  saveAutoRefreshToStorage()
  if (enabled) startAutoRefreshTimer()
  else {
    stopAutoRefreshTimer()
    autoRefreshCountdown.value = 0
  }
}

function setAutoRefreshInterval(seconds: (typeof autoRefreshIntervals)[number]) {
  autoRefreshIntervalSeconds.value = seconds
  saveAutoRefreshToStorage()
  if (autoRefreshEnabled.value) {
    autoRefreshCountdown.value = seconds
    startAutoRefreshTimer()
  }
}

function loadSavedAutoSort() {
  try {
    const saved = localStorage.getItem(AUTO_SORT_STORAGE_KEY)
    if (!saved) return
    const parsed = JSON.parse(saved) as { enabled?: boolean }
    autoSortEnabled.value = parsed.enabled === true
  } catch {
    // ignore
  }
}

function saveAutoSortToStorage() {
  try {
    localStorage.setItem(AUTO_SORT_STORAGE_KEY, JSON.stringify({ enabled: autoSortEnabled.value }))
  } catch {
    // ignore
  }
}

function setAutoSortEnabled(enabled: boolean) {
  autoSortEnabled.value = enabled
  saveAutoSortToStorage()
}

async function handleManualRefresh() {
  if (refreshing.value) return
  await Promise.all([loadUser({ silent: true }), refreshAll({ silent: true })])
}

async function handleAutoRefresh() {
  if (refreshing.value || loading.value || autoSorting.value) return
  await Promise.all([loadUser({ silent: true }), refreshAll({ silent: true })])
  if (autoSortEnabled.value) {
    await handlePoolAutoSort({ silent: true })
  }
}

async function saveFailoverToggle(on: boolean) {
  try {
    const current = await adminAPI.settings.getQualityHardCloseSettings()
    const updated = await adminAPI.settings.updateQualityHardCloseSettings({
      ...current,
      schedule_use_failover_error_rate: on
    })
    scheduleUseFailover.value = updated.schedule_use_failover_error_rate === true
    accountQualityWindowN.value = resolveAccountQualityWindowN(updated)
    appStore.showSuccess(t('admin.settings.qualityHardClose.saved'))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.qualityHardClose.saveFailed')))
  }
}

onMounted(() => {
  loadSavedAutoRefresh()
  loadSavedAutoSort()
  if (autoRefreshEnabled.value) startAutoRefreshTimer()
  void loadUser()
  void adminAPI.settings.getQualityHardCloseSettings().then((settings) => {
    scheduleUseFailover.value = settings.schedule_use_failover_error_rate === true
    accountQualityWindowN.value = resolveAccountQualityWindowN(settings)
  }).catch(() => undefined)
})

onUnmounted(() => {
  stopAutoRefreshTimer()
})
</script>

<style scoped>
.smart-schedule-pool-table-scroll :deep(.table-wrapper) {
  max-height: 70vh;
}
</style>
