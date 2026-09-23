<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400"
          >
            <Icon name="chart" size="sm" />
          </span>
          {{ t('admin.channelLoadtest.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.channelLoadtest.description') }}
        </p>
      </header>

      <div class="grid gap-6 xl:grid-cols-[minmax(280px,380px)_1fr]">
        <section class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-4 inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 dark:border-dark-700 dark:bg-dark-900">
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium"
              :class="source === 'account' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-600 dark:text-dark-300'"
              @click="source = 'account'"
            >
              {{ t('admin.channelLoadtest.sourceAccount') }}
            </button>
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium"
              :class="source === 'manual' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-600 dark:text-dark-300'"
              @click="source = 'manual'"
            >
              {{ t('admin.channelLoadtest.sourceManual') }}
            </button>
          </div>

          <div v-if="source === 'account'" class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.account') }}
            </label>
            <input
              v-model="accountQuery"
              type="search"
              class="input"
              :placeholder="t('admin.channelLoadtest.accountPlaceholder')"
              @input="searchAccounts"
            />
            <select v-model.number="accountId" class="input">
              <option :value="0">{{ t('admin.channelLoadtest.accountPlaceholder') }}</option>
              <option v-for="acc in accounts" :key="acc.id" :value="acc.id">
                #{{ acc.id }} {{ acc.name }} ({{ acc.platform }}/{{ acc.type }})
              </option>
            </select>
            <p v-if="!accounts.length && accountQuery" class="text-xs text-gray-500">
              {{ t('admin.channelLoadtest.noAccounts') }}
            </p>
          </div>

          <div v-else class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.baseUrl') }}
            </label>
            <input v-model="baseUrl" class="input" placeholder="https://api.example.com" />
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.apiKey') }}
            </label>
            <input v-model="apiKey" class="input" type="password" autocomplete="off" />
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.apiKeyHint') }}</p>
          </div>

          <div class="mt-4 space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.proxy') }}
            </label>
            <select v-model.number="proxyId" class="input" :disabled="proxiesLoading">
              <option :value="0">{{ t('admin.channelLoadtest.noProxy') }}</option>
              <option v-for="proxy in proxies" :key="proxy.id" :value="proxy.id">
                {{ proxy.name }} ({{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }})
              </option>
            </select>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.proxyHint') }}</p>
          </div>

          <div class="mt-4 space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.model') }}
            </label>
            <input v-model="models" class="input" placeholder="kimi-k3, glm-5.3" />
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.modelHint') }}</p>

            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.channelLoadtest.profile') }}
            </label>
            <select v-model="profile" class="input">
              <option value="smoke">{{ t('admin.channelLoadtest.profileSmoke') }}</option>
              <option value="user363">{{ t('admin.channelLoadtest.profileUser363') }}</option>
              <option value="user363-stream">{{ t('admin.channelLoadtest.profileStream') }}</option>
              <option value="user363-sync">{{ t('admin.channelLoadtest.profileSync') }}</option>
              <option value="user363-sla">{{ t('admin.channelLoadtest.profileSla') }}</option>
            </select>
            <button type="button" class="btn btn-secondary w-full text-sm" data-testid="apply-sla-preset" @click="applySheetPreset">
              {{ t('admin.channelLoadtest.applySheetPreset') }}
            </button>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.sheetPresetHint') }}</p>

            <div class="space-y-2">
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.tiers') }}
                </span>
                <button type="button" class="text-xs text-gray-500 hover:text-gray-800 dark:hover:text-gray-200" data-testid="add-tier" @click="addTier">
                  {{ t('admin.channelLoadtest.addTier') }}
                </button>
              </div>
              <div v-for="(row, index) in tiers" :key="index" class="grid grid-cols-[minmax(0,1fr)_5.5rem_auto] gap-2">
                <input
                  v-model.number="row.inputTokens"
                  type="number"
                  min="1"
                  max="400000"
                  class="input"
                  data-testid="tier-input"
                  :placeholder="t('admin.channelLoadtest.tierInput')"
                />
                <input
                  v-model.number="row.count"
                  type="number"
                  min="1"
                  max="500"
                  class="input"
                  data-testid="tier-count"
                  :placeholder="t('admin.channelLoadtest.tierCount')"
                />
                <button type="button" class="btn btn-secondary px-2 text-xs" @click="removeTier(index)">
                  {{ t('admin.channelLoadtest.removeTier') }}
                </button>
              </div>
              <p v-if="tiers.length" class="text-xs text-gray-500">
                {{ t('admin.channelLoadtest.tierHint', { n: tierRequestCount }) }}
              </p>
              <button
                v-if="tiers.length"
                type="button"
                class="text-xs text-gray-500 hover:text-gray-800 dark:hover:text-gray-200"
                data-testid="clear-tiers"
                @click="clearTiers"
              >
                {{ t('admin.channelLoadtest.clearTiers') }}
              </button>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.apiMode') }}
                </label>
                <select v-model="apiMode" class="input">
                  <option value="chat_completions">{{ t('admin.channelLoadtest.apiModeCC') }}</option>
                  <option value="responses" :disabled="kimiOnly">{{ t('admin.channelLoadtest.apiModeRE') }}</option>
                </select>
                <p v-if="kimiOnly" class="mt-1 text-xs text-gray-500">{{ t('admin.channelLoadtest.apiModeKimiHint') }}</p>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.streamMode') }}
                </label>
                <select v-model="streamMode" class="input">
                  <option value="auto">{{ t('admin.channelLoadtest.streamModeAuto') }}</option>
                  <option value="stream">{{ t('admin.channelLoadtest.streamModeStream') }}</option>
                  <option value="sync">{{ t('admin.channelLoadtest.streamModeSync') }}</option>
                </select>
              </div>
            </div>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.streamModeHint') }}</p>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.concurrency') }}
                </label>
                <input v-model.number="concurrency" type="number" min="1" max="80" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.total') }}
                </label>
                <input v-model.number="total" data-testid="loadtest-total" type="number" min="1" max="500" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.maxTokens') }}
                </label>
                <input v-model.number="maxTokens" type="number" min="0" max="8000" class="input" />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.sizeCap') }}
                </label>
                <input v-model.number="sizeCap" type="number" min="0" max="400000" class="input" />
              </div>
              <div class="col-span-2">
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.channelLoadtest.inputTokens') }}
                </label>
                <input v-model.number="inputTokens" type="number" min="0" max="400000" class="input" />
                <p class="mt-1 text-xs text-gray-500">{{ t('admin.channelLoadtest.inputTokensHint') }}</p>
              </div>
            </div>
            <p class="text-xs text-gray-500">{{ t('admin.channelLoadtest.sizeCapHint') }}</p>

            <select v-model="tools" class="input">
              <option value="auto">{{ t('admin.channelLoadtest.toolsAuto') }}</option>
              <option value="off">{{ t('admin.channelLoadtest.toolsOff') }}</option>
              <option value="coding">{{ t('admin.channelLoadtest.toolsCoding') }}</option>
            </select>

            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input v-model="confirmCost" type="checkbox" />
              {{ t('admin.channelLoadtest.confirmCost') }}
            </label>
          </div>

          <div class="mt-5 flex gap-2">
            <button
              type="button"
              class="btn btn-primary flex-1"
              :disabled="busy || running"
              @click="startRun"
            >
              {{ t('admin.channelLoadtest.start') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="!running"
              @click="stopRun"
            >
              {{ t('admin.channelLoadtest.stop') }}
            </button>
          </div>
        </section>

        <section class="space-y-4">
          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ running ? t('admin.channelLoadtest.running') : t('admin.channelLoadtest.live') }}
              </h2>
              <span v-if="snap" class="text-xs text-gray-500">
                {{ snap.status }} · {{ snap.id?.slice(0, 8) }}
                <template v-if="snap.api_mode"> · {{ snap.api_mode }}</template>
                <template v-if="snap.stream_mode"> · {{ snap.stream_mode }}</template>
                <template v-if="snap.proxy_name"> · {{ snap.proxy_name }}</template>
              </span>
            </div>
            <p v-if="snap?.tiers?.length" class="mt-3 text-xs text-gray-500">
              {{ t('admin.channelLoadtest.appliedTiers') }}
              <template v-for="(tier, index) in snap.tiers" :key="`${tier.input_tokens}-${index}`">
                {{ index ? ' · ' : '' }}{{ tier.count }}×{{ fmtTok(tier.input_tokens) }}
              </template>
            </p>
            <div v-if="snap" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">in-flight</div>
                <div class="text-lg font-semibold">{{ snap.inflight }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.peak') }}</div>
                <div class="text-lg font-semibold">{{ snap.peak }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.done') }}</div>
                <div class="text-lg font-semibold">{{ snap.done }} / {{ snap.total }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
                <div class="text-xs text-gray-500">{{ t('admin.channelLoadtest.success') }}</div>
                <div class="text-lg font-semibold">{{ snap.success_rate.toFixed(1) }}%</div>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900" :title="t('admin.channelLoadtest.rpmHint')">
                <div class="text-xs text-gray-500">RPM</div>
                <div class="text-lg font-semibold tabular-nums">{{ snap.rpm ?? 0 }}</div>
                <div class="text-[11px] text-gray-500">
                  {{ t('admin.channelLoadtest.rpmDetail', { peak: snap.rpm_peak ?? 0, avg: (snap.rpm_avg ?? 0).toFixed(1) }) }}
                </div>
              </div>
            </div>
            <p v-else class="mt-3 text-sm text-gray-500">{{ t('admin.channelLoadtest.idle') }}</p>
            <p v-if="snap?.error" class="mt-3 text-sm text-red-600">{{ snap.error }}</p>
            <p
              v-if="snap && snap.model_missing_requests > 0"
              class="mt-3 text-sm text-amber-700 dark:text-amber-400"
            >
              {{ t('admin.channelLoadtest.emptyModelWarn', { n: snap.model_missing_requests }) }}
            </p>
          </div>

          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.channelLoadtest.dataTitle') }}
            </h2>
            <p class="mt-1 text-xs text-gray-500">{{ t('admin.channelLoadtest.dataProfileHint') }}</p>
            <table class="mt-3 w-full text-left text-sm">
              <thead>
                <tr class="text-xs text-gray-500">
                  <th class="py-1"></th>
                  <th class="py-1">p50</th>
                  <th class="py-1">p90</th>
                  <th class="py-1">p99</th>
                  <th class="py-1">avg</th>
                </tr>
              </thead>
              <tbody>
                <tr class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ t('admin.channelLoadtest.sheetInput') }}</td>
                  <td>50K</td><td>160K</td><td>380K</td><td>80K</td>
                </tr>
                <tr class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ t('admin.channelLoadtest.builtInput') }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.target_input?.p50) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.target_input?.p90) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.target_input?.p99) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.target_input?.avg) }}</td>
                </tr>
                <tr class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ t('admin.channelLoadtest.usageInput') }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_input?.p50) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_input?.p90) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_input?.p99) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_input?.avg) }}</td>
                </tr>
                <tr class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ t('admin.channelLoadtest.sheetOutput') }}</td>
                  <td>0.2K</td><td>1.3K</td><td>7K</td><td>0.6K</td>
                </tr>
                <tr class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5">{{ t('admin.channelLoadtest.usageOutput') }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_output?.p50) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_output?.p90) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_output?.p99) }}</td>
                  <td>{{ fmtTok(snap?.data_profile?.usage_output?.avg) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.channelLoadtest.slaTitle') }}
              </h2>
              <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-0.5 text-xs dark:border-dark-700 dark:bg-dark-900">
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 'ms' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 'ms'"
                >
                  {{ t('admin.channelLoadtest.timeUnitMs') }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 's' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 's'"
                >
                  {{ t('admin.channelLoadtest.timeUnitS') }}
                </button>
              </div>
            </div>
            <table v-if="snap?.sla?.length" class="mt-3 w-full text-left text-sm">
              <thead>
                <tr class="text-xs text-gray-500">
                  <th class="py-1">{{ t('admin.channelLoadtest.inputBand') }}</th>
                  <th class="py-1">metric</th>
                  <th class="py-1">want</th>
                  <th class="py-1">got</th>
                  <th class="py-1"> </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in snap.sla" :key="row.name" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="py-1.5 text-gray-500">{{ row.band || '-' }}</td>
                  <td class="py-1.5">{{ row.name }}</td>
                  <td class="py-1.5 text-gray-500">{{ formatSlaWant(row) }}</td>
                  <td class="py-1.5">{{ formatSlaGot(row) }}</td>
                  <td class="py-1.5">
                    <span
                      class="rounded px-1.5 py-0.5 text-xs font-semibold"
                      :class="row.skip ? 'bg-gray-100 text-gray-500' : row.pass ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'"
                    >
                      {{ row.skip ? 'n/a' : row.pass ? t('admin.channelLoadtest.pass') : t('admin.channelLoadtest.fail') }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-else class="mt-2 text-sm text-gray-500">{{ t('admin.channelLoadtest.noResults') }}</p>
          </div>

          <div class="overflow-hidden rounded-3xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3 px-5 py-3">
              <div class="text-sm font-semibold">
                {{ t('admin.channelLoadtest.results') }}
                <span v-if="snap?.results?.length" class="ml-1 font-normal text-gray-500">
                  ({{ visibleResults.length }}/{{ snap.results.length }})
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  class="rounded-lg border px-2 py-1 text-xs font-medium"
                  :class="failOnly ? 'border-red-300 bg-red-50 text-red-700' : 'border-gray-200 text-gray-600 dark:border-dark-600'"
                  @click="failOnly = !failOnly"
                >
                  {{ t('admin.channelLoadtest.failOnly') }}
                </button>
                <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-0.5 text-xs dark:border-dark-700 dark:bg-dark-900">
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 'ms' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 'ms'"
                >
                  {{ t('admin.channelLoadtest.timeUnitMs') }}
                </button>
                <button
                  type="button"
                  class="rounded-md px-2 py-1 font-medium"
                  :class="timeUnit === 's' ? 'bg-white shadow-sm dark:bg-dark-800' : 'text-gray-500'"
                  @click="timeUnit = 's'"
                >
                  {{ t('admin.channelLoadtest.timeUnitS') }}
                </button>
                </div>
              </div>
            </div>
            <div class="max-h-[28rem] overflow-auto">
              <table class="min-w-full text-left text-xs">
                <thead class="sticky top-0 bg-gray-50 text-gray-500 dark:bg-dark-900">
                  <tr>
                    <th class="px-3 py-2">#</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.inputTokens') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.requestedModel') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.outcome') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.error') }}</th>
                    <th class="px-3 py-2" :title="t('admin.channelLoadtest.ttftHint')">{{ t('admin.channelLoadtest.ttft') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.duration') }}</th>
                    <th class="px-3 py-2" :title="t('usage.tokenRateHint')">{{ t('usage.inputPerSecondColumn') }}</th>
                    <th class="px-3 py-2" :title="t('usage.tokenRateHint')">{{ t('usage.outputPerSecondColumn') }}</th>
                    <th class="px-3 py-2">{{ t('admin.channelLoadtest.tpot') }}</th>
                    <th class="px-3 py-2">model</th>
                  </tr>
                </thead>
                <tbody>
                  <template v-for="row in visibleResults" :key="row.seq">
                    <tr
                      class="cursor-pointer border-t border-gray-100 dark:border-dark-700"
                      :class="row.outcome !== 'success' ? 'bg-red-50/60 dark:bg-red-950/20' : ''"
                      @click="toggleError(row.seq)"
                    >
                      <td class="px-3 py-1.5">{{ row.seq }}</td>
                      <td class="px-3 py-1.5">{{ fmtTok(row.target_input_tokens) }}{{ row.input_band ? ` (${row.input_band})` : '' }}</td>
                      <td class="px-3 py-1.5">{{ row.requested_model }}</td>
                      <td class="px-3 py-1.5">
                        <span class="font-medium">{{ row.outcome }}</span>
                        <span v-if="row.status_code" class="ml-1 text-gray-500">{{ row.status_code }}</span>
                      </td>
                      <td class="max-w-xs px-3 py-1.5">
                        <div v-if="row.error_category || row.error_message" class="truncate text-red-700 dark:text-red-300" :title="errorLine(row)">
                          <span v-if="row.error_category" class="font-medium">{{ row.error_category }}</span>
                          <span v-if="row.error_message"> {{ row.error_message }}</span>
                        </div>
                        <span v-else class="text-gray-400">-</span>
                      </td>
                      <td class="px-3 py-1.5" :title="ttftTitle(row)">{{ formatMs(displayedTTFT(row)) }}</td>
                      <td class="px-3 py-1.5">{{ formatMs(row.duration_ms) }}</td>
                      <td class="px-3 py-1.5 tabular-nums">{{ formatTps(row.prompt_tokens, row.duration_ms) }}</td>
                      <td class="px-3 py-1.5 tabular-nums">{{ formatTps(row.completion_tokens, row.duration_ms) }}</td>
                      <td class="px-3 py-1.5">{{ row.tpot_tok_s ? row.tpot_tok_s.toFixed(1) : '-' }}</td>
                      <td class="px-3 py-1.5">{{ row.response_model || (row.model_missing_chunks ? 'missing' : '-') }}</td>
                    </tr>
                    <tr v-if="expandedSeq === row.seq && (row.error_message || row.contract_issues?.length || row.request_id)" class="border-t border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-900">
                      <td colspan="11" class="px-4 py-2">
                        <div v-if="row.request_id" class="mb-1 font-mono text-[11px] text-gray-500">{{ row.request_id }}</div>
                        <pre v-if="row.error_message" class="whitespace-pre-wrap break-all text-xs text-red-800 dark:text-red-200">{{ row.error_message }}</pre>
                        <div v-if="row.contract_issues?.length" class="mt-1 text-xs text-amber-700">{{ row.contract_issues.join(', ') }}</div>
                      </td>
                    </tr>
                  </template>
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { LoadtestAPIMode, LoadtestProfile, LoadtestResult, LoadtestSLAVerdict, LoadtestSnapshot, LoadtestStreamMode, LoadtestTier } from '@/api/admin/channelLoadtest'
import { list as listAccounts } from '@/api/admin/accounts'
import { getAll as listProxies } from '@/api/admin/proxies'
import type { Account, Proxy } from '@/types'
import { tokensPerSecond } from '@/utils/latencyHealth'

const { t } = useI18n()
const appStore = useAppStore()

const source = ref<'account' | 'manual'>('account')
const accountQuery = ref('')
const accountId = ref(0)
const accounts = ref<Account[]>([])
const proxyId = ref(0)
const proxies = ref<Proxy[]>([])
const proxiesLoading = ref(false)
const baseUrl = ref('')
const apiKey = ref('')
const models = ref('kimi-k3')
const profile = ref<LoadtestProfile>('smoke')
const apiMode = ref<LoadtestAPIMode>('chat_completions')
const kimiOnly = computed(() => {
  const list = models.value.split(',').map((item) => item.trim()).filter(Boolean)
  return list.length > 0 && list.every(isKimiNativeModel)
})
const streamMode = ref<LoadtestStreamMode>('auto')
const concurrency = ref(20)
const total = ref(40)
const maxTokens = ref(256)
const sizeCap = ref(80000)
const inputTokens = ref(0)
interface TierDraft {
  inputTokens: number | string
  count: number | string
}
const tiers = ref<TierDraft[]>([])
const tools = ref('auto')
const confirmCost = ref(false)
const busy = ref(false)
const snap = ref<LoadtestSnapshot | null>(null)
const timeUnit = ref<'ms' | 's'>((localStorage.getItem('channel-loadtest-time-unit') as 'ms' | 's') || 'ms')
const failOnly = ref(false)
const expandedSeq = ref<number | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const running = computed(() => snap.value?.status === 'running' || snap.value?.status === 'stopping')
const tierRequestCount = computed(() =>
  tiers.value.reduce((sum, row) => {
    const count = tierNumber(row.count)
    return sum + (count != null && count >= 1 ? count : 0)
  }, 0)
)
const visibleResults = computed(() => {
  const rows = snap.value?.results || []
  if (!failOnly.value) return rows
  return rows.filter((row) => row.outcome !== 'success')
})

function errorLine(row: LoadtestResult) {
  return [row.error_category, row.error_message].filter(Boolean).join(' ')
}

function toggleError(seq: number) {
  expandedSeq.value = expandedSeq.value === seq ? null : seq
}

watch(timeUnit, (unit) => {
  localStorage.setItem('channel-loadtest-time-unit', unit)
})

function displayedTTFT(row: LoadtestResult) {
  return row.first_token_ms || row.first_content_ms
}

function ttftTitle(row: LoadtestResult) {
  if (row.first_token_ms && row.first_content_ms && row.first_content_ms - row.first_token_ms > 50) {
    return t('admin.channelLoadtest.ttftAnswer', { n: formatMs(row.first_content_ms) })
  }
  return t('admin.channelLoadtest.ttftHint')
}

function formatTps(tokens?: number, durationMs?: number) {
  const rate = tokensPerSecond(tokens ?? 0, durationMs ?? 0)
  return rate == null ? '-' : String(rate)
}

function formatMs(ms?: number) {
  if (ms == null || ms <= 0) return '-'
  if (timeUnit.value === 's') {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

function formatSlaWant(row: LoadtestSLAVerdict) {
  if (row.metric === 'ttft' && row.want_value) {
    return `<${formatMs(row.want_value)}`
  }
  return row.want
}

function fmtTok(n?: number) {
  if (!n) return '-'
  if (n >= 1000) {
    const k = n / 1000
    return k >= 10 ? `${Math.round(k)}K` : `${k.toFixed(1)}K`
  }
  return String(n)
}

function tierNumber(value: number | string | null | undefined): number | null {
  if (value === '' || value == null) return null
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n)) return null
  return n
}

function addTier() {
  tiers.value.push({ inputTokens: '', count: '' })
}

function removeTier(index: number) {
  tiers.value.splice(index, 1)
}

function clearTiers() {
  tiers.value = []
}

function collectTiers(): LoadtestTier[] | null {
  const filled = tiers.value.filter((row) => tierNumber(row.inputTokens) != null || tierNumber(row.count) != null)
  if (!filled.length) return []
  if (filled.length > 64) {
    appStore.showError(t('admin.channelLoadtest.tierInvalid'))
    return null
  }
  const out: LoadtestTier[] = []
  for (const row of filled) {
    const input = tierNumber(row.inputTokens)
    const count = tierNumber(row.count)
    if (input == null || !Number.isInteger(input) || input < 1 || input > 400000 || count == null || !Number.isInteger(count) || count < 1) {
      appStore.showError(t('admin.channelLoadtest.tierInvalid'))
      return null
    }
    out.push({ input_tokens: input, count })
  }
  const sum = out.reduce((totalCount, row) => totalCount + row.count, 0)
  if (sum < 1 || sum > 500) {
    appStore.showError(t('admin.channelLoadtest.tierInvalid'))
    return null
  }
  return out
}

function applySheetPreset() {
  profile.value = 'user363-sla'
  streamMode.value = 'stream'
  sizeCap.value = 0
  inputTokens.value = 0
  tools.value = 'off'
  timeUnit.value = 's'
  concurrency.value = 50
  total.value = 100
  tiers.value = [
    { inputTokens: 50000, count: 50 },
    { inputTokens: 80000, count: 38 },
    { inputTokens: 160000, count: 10 },
    { inputTokens: 380000, count: 2 }
  ]
}

watch(profile, (p) => {
  if (p === 'user363-sla') {
    sizeCap.value = 0
    streamMode.value = 'stream'
  }
})

watch(kimiOnly, (only) => {
  if (only) apiMode.value = 'chat_completions'
})

function isKimiNativeModel(model: string) {
  const bare = model.trim().toLowerCase().split('/').pop() || ''
  return bare === 'kimi' || bare.startsWith('kimi-')
}

function formatSlaGot(row: LoadtestSLAVerdict) {
  if (row.skip) return 'n/a'
  if (row.metric === 'ttft') {
    const n = row.samples ? ` (n=${row.samples})` : ''
    return `${formatMs(row.got_value)}${n}`
  }
  return row.got
}

async function loadProxies() {
  proxiesLoading.value = true
  try {
    proxies.value = await listProxies()
  } catch {
    proxies.value = []
  } finally {
    proxiesLoading.value = false
  }
}

watch(accountId, (id) => {
  const acc = accounts.value.find((item) => item.id === id)
  proxyId.value = acc?.proxy_id && acc.proxy_id > 0 ? acc.proxy_id : 0
})

async function loadAccounts() {
  try {
    const res = await listAccounts(1, 30, {
      search: accountQuery.value || undefined,
      type: 'apikey',
      lite: '1'
    })
    accounts.value = res.items || []
  } catch {
    accounts.value = []
  }
}

function searchAccounts() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    void loadAccounts()
  }, 250)
}

async function startRun() {
  busy.value = true
  try {
    const plannedTiers = collectTiers()
    if (plannedTiers == null) return
    const payload: Parameters<typeof adminAPI.channelLoadtest.start>[0] = {
      model: models.value.split(',')[0]?.trim(),
      models: models.value,
      profile: profile.value,
      api_mode: apiMode.value,
      stream_mode: streamMode.value,
      concurrency: concurrency.value,
      total: total.value,
      max_tokens: maxTokens.value,
      size_cap: sizeCap.value,
      input_tokens: inputTokens.value,
      tools: tools.value,
      confirm_cost: confirmCost.value
    }
    if (plannedTiers.length) payload.tiers = plannedTiers
    if (source.value === 'account') {
      if (!accountId.value) {
        appStore.showError(t('admin.channelLoadtest.account'))
        return
      }
      payload.account_id = accountId.value
    } else {
      payload.base_url = baseUrl.value
      payload.api_key = apiKey.value
    }
    payload.proxy_id = proxyId.value > 0 ? proxyId.value : 0
    snap.value = await adminAPI.channelLoadtest.start(payload)
    startPolling()
  } catch (err) {
    appStore.showError((err as Error).message || t('admin.channelLoadtest.startError'))
  } finally {
    busy.value = false
  }
}

async function stopRun() {
  if (!snap.value?.id) return
  try {
    snap.value = await adminAPI.channelLoadtest.stop(snap.value.id)
  } catch (err) {
    appStore.showError((err as Error).message || t('admin.channelLoadtest.stopError'))
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    if (!snap.value?.id) return
    try {
      snap.value = await adminAPI.channelLoadtest.get(snap.value.id)
      if (!running.value) stopPolling()
    } catch {
      appStore.showError(t('admin.channelLoadtest.loadError'))
    }
  }, 500)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(async () => {
  await Promise.all([loadAccounts(), loadProxies()])
  try {
    const latest = await adminAPI.channelLoadtest.latest()
    if (latest) {
      snap.value = latest
      if (latest.status === 'running' || latest.status === 'stopping') startPolling()
    }
  } catch {
    /* ignore */
  }
})

onUnmounted(() => {
  stopPolling()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>
