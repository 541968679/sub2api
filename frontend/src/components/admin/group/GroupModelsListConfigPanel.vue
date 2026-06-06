<template>
  <div class="border-t pt-4">
    <div class="mb-3 flex items-start justify-between gap-3">
      <div>
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t("admin.groups.modelsList.title") }}
        </label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.groups.modelsList.hint") }}
        </p>
      </div>
      <button
        type="button"
        :aria-pressed="state.enabled"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors',
          state.enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600',
        ]"
        @click="$emit('toggle-enabled')"
      >
        <span
          :class="[
            'inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform',
            state.enabled ? 'translate-x-6' : 'translate-x-1',
          ]"
        />
      </button>
    </div>

    <div
      v-if="state.enabled"
      class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50/50 dark:border-dark-600 dark:bg-dark-800/40"
    >
      <div
        v-if="!loading && state.items.length > 0"
        class="flex items-center justify-between gap-2 border-b border-gray-200 bg-gray-50 px-3 py-2 text-xs dark:border-dark-600 dark:bg-dark-800"
      >
        <span class="text-gray-500 dark:text-gray-400">
          {{
            t("admin.groups.modelsList.selectedCount", {
              selected: selectedCount,
              total: state.items.length,
            })
          }}
        </span>
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            class="rounded px-2 py-1 font-medium text-primary-600 transition-colors hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/20"
            @click="$emit('select-all')"
          >
            {{ t("admin.groups.modelsList.selectAll") }}
          </button>
          <button
            type="button"
            class="rounded px-2 py-1 font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
            @click="$emit('invert-selection')"
          >
            {{ t("admin.groups.modelsList.invert") }}
          </button>
        </div>
      </div>

      <div class="max-h-64 space-y-2 overflow-y-auto p-2">
        <p v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.groups.modelsList.loading") }}
        </p>
        <p
          v-else-if="state.items.length === 0"
          class="text-xs text-gray-500 dark:text-gray-400"
        >
          {{ t("admin.groups.modelsList.empty") }}
        </p>
        <div
          v-for="(item, index) in state.items"
          :key="item.id"
          class="flex items-center gap-2 rounded border border-gray-200 bg-white px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
        >
          <input
            :checked="item.selected"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
            @change="$emit('toggle-item', item.id)"
          />
          <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
            {{ item.id }}
          </span>
          <button
            type="button"
            :disabled="index === 0"
            class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t('admin.groups.modelsList.moveUp')"
            @click="$emit('move-item', index, index - 1)"
          >
            <Icon name="arrowUp" size="sm" />
          </button>
          <button
            type="button"
            :disabled="index === state.items.length - 1"
            class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t('admin.groups.modelsList.moveDown')"
            @click="$emit('move-item', index, index + 1)"
          >
            <Icon name="arrowDown" size="sm" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import Icon from "@/components/icons/Icon.vue";
import type { ModelsListState } from "@/views/admin/groupsModelsList";

const props = defineProps<{
  state: ModelsListState;
  loading?: boolean;
}>();

defineEmits<{
  "toggle-enabled": [];
  "select-all": [];
  "invert-selection": [];
  "toggle-item": [modelID: string];
  "move-item": [fromIndex: number, toIndex: number];
}>();

const { t } = useI18n();

const selectedCount = computed(
  () => props.state.items.filter((item) => item.selected).length,
);
</script>
