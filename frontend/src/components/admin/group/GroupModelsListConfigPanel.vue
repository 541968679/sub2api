<template>
  <div class="border-t pt-4">
    <div class="mb-3 flex items-start justify-between gap-3">
      <div>
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t(`${i18nPrefix}.title`) }}
        </label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t(`${i18nPrefix}.hint`) }}
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
        class="border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="flex items-center justify-between gap-2 text-xs">
          <span class="text-gray-500 dark:text-gray-400">
            {{
              t(`${i18nPrefix}.selectedCount`, {
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
              {{ t(`${i18nPrefix}.selectAll`) }}
            </button>
            <button
              type="button"
              class="rounded px-2 py-1 font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
              @click="$emit('invert-selection')"
            >
              {{ t(`${i18nPrefix}.invert`) }}
            </button>
          </div>
        </div>
        <input
          v-model="filterQuery"
          type="search"
          autocomplete="off"
          spellcheck="false"
          class="input mt-2"
          :placeholder="t(`${i18nPrefix}.filterPlaceholder`)"
        />
      </div>

      <div class="max-h-64 space-y-2 overflow-y-auto p-2">
        <p v-if="loading" class="text-xs text-gray-500 dark:text-gray-400">
          {{ t(`${i18nPrefix}.loading`) }}
        </p>
        <p
          v-else-if="state.items.length === 0"
          class="text-xs text-gray-500 dark:text-gray-400"
        >
          {{ t(`${i18nPrefix}.empty`) }}
        </p>
        <p
          v-else-if="visibleItems.length === 0"
          class="text-xs text-gray-500 dark:text-gray-400"
        >
          {{ t(`${i18nPrefix}.filterEmpty`) }}
        </p>
        <div
          v-for="entry in visibleItems"
          :key="entry.item.id"
          class="flex items-center gap-2 rounded border border-gray-200 bg-white px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
        >
          <input
            :checked="entry.item.selected"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
            @change="$emit('toggle-item', entry.item.id)"
          />
          <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
            {{ entry.item.id }}
          </span>
          <button
            type="button"
            :disabled="entry.index === 0 || filterActive"
            class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t(`${i18nPrefix}.moveUp`)"
            @click="$emit('move-item', entry.index, entry.index - 1)"
          >
            <Icon name="arrowUp" size="sm" />
          </button>
          <button
            type="button"
            :disabled="entry.index === state.items.length - 1 || filterActive"
            class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-40 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t(`${i18nPrefix}.moveDown`)"
            @click="$emit('move-item', entry.index, entry.index + 1)"
          >
            <Icon name="arrowDown" size="sm" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import Icon from "@/components/icons/Icon.vue";
import type { ModelsListState } from "@/views/admin/groupsModelsList";
import type { ModelAllowlistState } from "@/views/admin/groupModelAllowlist";

const props = withDefaults(
  defineProps<{
    state: ModelsListState | ModelAllowlistState;
    loading?: boolean;
    i18nPrefix?: string;
  }>(),
  {
    i18nPrefix: "admin.groups.modelsList",
  },
);

const i18nPrefix = computed(() => props.i18nPrefix);

defineEmits<{
  "toggle-enabled": [];
  "select-all": [];
  "invert-selection": [];
  "toggle-item": [modelID: string];
  "move-item": [fromIndex: number, toIndex: number];
}>();

const { t } = useI18n();

const filterQuery = ref("");
watch(
  () => props.state.items,
  () => {
    filterQuery.value = "";
  },
);

const filterActive = computed(() => filterQuery.value.trim() !== "");

const visibleItems = computed(() => {
  const query = filterQuery.value.trim().toLowerCase();
  return props.state.items
    .map((item, index) => ({ item, index }))
    .filter(
      ({ item }) => query === "" || item.id.toLowerCase().includes(query),
    );
});

const selectedCount = computed(
  () => props.state.items.filter((item) => item.selected).length,
);
</script>
