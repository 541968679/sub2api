<template>
  <div class="space-y-6">
    <p class="text-sm text-gray-500 dark:text-gray-400">
      {{ t("admin.modelConfig.catalog.hint") }}
    </p>

    <div class="grid gap-6 lg:grid-cols-2">
      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <h2 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t("admin.modelConfig.catalog.displayTitle") }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.modelConfig.catalog.displayHint") }}
        </p>
        <div class="mt-3 flex gap-2">
          <input
            v-model="displayDraft"
            type="text"
            class="input flex-1 text-sm"
            :placeholder="t('admin.modelConfig.catalog.addPlaceholder')"
            @keydown.enter.prevent="addDisplay"
          />
          <button type="button" class="btn btn-secondary text-sm" @click="addDisplay">
            {{ t("admin.modelConfig.catalog.add") }}
          </button>
        </div>
        <ul class="mt-3 max-h-80 space-y-2 overflow-y-auto">
          <li
            v-for="(id, index) in displayModels"
            :key="'d-' + id"
            class="flex items-center gap-2 rounded border border-gray-200 px-3 py-2 dark:border-gray-600"
          >
            <span class="min-w-0 flex-1 truncate text-sm text-gray-800 dark:text-gray-200">{{ id }}</span>
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              :disabled="index === 0"
              @click="moveDisplay(index, index - 1)"
            >
              ↑
            </button>
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              :disabled="index === displayModels.length - 1"
              @click="moveDisplay(index, index + 1)"
            >
              ↓
            </button>
            <button
              type="button"
              class="rounded p-1 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20"
              @click="removeDisplay(id)"
            >
              ×
            </button>
          </li>
        </ul>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
        <h2 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t("admin.modelConfig.catalog.whitelistTitle") }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.modelConfig.catalog.whitelistHint") }}
        </p>
        <div class="mt-3 flex gap-2">
          <input
            v-model="whitelistDraft"
            type="text"
            class="input flex-1 text-sm"
            :placeholder="t('admin.modelConfig.catalog.addPlaceholder')"
            @keydown.enter.prevent="addWhitelist"
          />
          <button type="button" class="btn btn-secondary text-sm" @click="addWhitelist">
            {{ t("admin.modelConfig.catalog.add") }}
          </button>
        </div>
        <ul class="mt-3 max-h-80 space-y-2 overflow-y-auto">
          <li
            v-for="(id, index) in whitelistModels"
            :key="'w-' + id"
            class="flex items-center gap-2 rounded border border-gray-200 px-3 py-2 dark:border-gray-600"
          >
            <span class="min-w-0 flex-1 truncate text-sm text-gray-800 dark:text-gray-200">{{ id }}</span>
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              :disabled="index === 0"
              @click="moveWhitelist(index, index - 1)"
            >
              ↑
            </button>
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              :disabled="index === whitelistModels.length - 1"
              @click="moveWhitelist(index, index + 1)"
            >
              ↓
            </button>
            <button
              type="button"
              class="rounded p-1 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20"
              @click="removeWhitelist(id)"
            >
              ×
            </button>
          </li>
        </ul>
      </section>
    </div>

    <div class="flex justify-end">
      <button
        type="button"
        class="btn btn-primary text-sm"
        :disabled="saving"
        @click="save"
      >
        {{ saving ? t("admin.modelConfig.catalog.saving") : t("admin.modelConfig.catalog.save") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useAppStore } from "@/stores/app";
import {
  getOpenAIModelCatalog,
  previewOpenAIModelCatalogMerge,
  updateOpenAIModelCatalog,
} from "@/api/admin/modelCatalog";
import {
  addDisplayModel,
  moveModelID,
  normalizeModelIDs,
  removeWhitelistModel,
} from "@/views/admin/openaiModelCatalog";

const { t } = useI18n();
const appStore = useAppStore();

const displayModels = ref<string[]>([]);
const whitelistModels = ref<string[]>([]);
const displayDraft = ref("");
const whitelistDraft = ref("");
const saving = ref(false);

onMounted(async () => {
  try {
    const catalog = await getOpenAIModelCatalog();
    displayModels.value = normalizeModelIDs(catalog.display_models ?? []);
    whitelistModels.value = normalizeModelIDs(catalog.whitelist_models ?? []);
  } catch (error) {
    appStore.showError(
      error instanceof Error ? error.message : t("admin.modelConfig.catalog.loadFailed"),
    );
  }
});

function addDisplay() {
  const next = addDisplayModel(displayModels.value, whitelistModels.value, displayDraft.value);
  displayModels.value = next.display;
  whitelistModels.value = next.whitelist;
  displayDraft.value = "";
}

function addWhitelist() {
  whitelistModels.value = normalizeModelIDs([
    ...whitelistModels.value,
    whitelistDraft.value,
  ]);
  whitelistDraft.value = "";
}

function removeDisplay(id: string) {
  displayModels.value = displayModels.value.filter((item) => item !== id);
}

function removeWhitelist(id: string) {
  const next = removeWhitelistModel(displayModels.value, whitelistModels.value, id);
  displayModels.value = next.display;
  whitelistModels.value = next.whitelist;
}

function moveDisplay(from: number, to: number) {
  displayModels.value = moveModelID(displayModels.value, from, to);
}

function moveWhitelist(from: number, to: number) {
  whitelistModels.value = moveModelID(whitelistModels.value, from, to);
}

async function save() {
  saving.value = true;
  try {
    const payload = {
      display_models: displayModels.value,
      whitelist_models: whitelistModels.value,
    };
    const preview = await previewOpenAIModelCatalogMerge(payload);
    const normalizedPayload = {
      display_models: normalizeModelIDs(preview.display_models ?? payload.display_models),
      whitelist_models: normalizeModelIDs(preview.whitelist_models ?? payload.whitelist_models),
    };
    displayModels.value = normalizedPayload.display_models;
    whitelistModels.value = normalizedPayload.whitelist_models;
    const count = preview.merged_account_count ?? 0;
    const confirmed = window.confirm(
      t("admin.modelConfig.catalog.confirmSave", { count }),
    );
    if (!confirmed) {
      return;
    }
    const saved = await updateOpenAIModelCatalog(normalizedPayload);
    displayModels.value = normalizeModelIDs(saved.display_models ?? []);
    whitelistModels.value = normalizeModelIDs(saved.whitelist_models ?? []);
    appStore.showSuccess(
      t("admin.modelConfig.catalog.saveSuccess", {
        count: saved.merged_account_count ?? count,
      }),
    );
  } catch (error) {
    appStore.showError(
      error instanceof Error ? error.message : t("admin.modelConfig.catalog.saveFailed"),
    );
  } finally {
    saving.value = false;
  }
}
</script>
