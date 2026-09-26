import type { ModelsListConfig } from "@/types";

export interface ModelsListItem {
  id: string;
  selected: boolean;
}

export interface ModelsListState {
  enabled: boolean;
  savedModels: string[];
  items: ModelsListItem[];
}

export function createModelsListState(
  config?: Partial<ModelsListConfig> | null,
): ModelsListState {
  return {
    enabled: config?.enabled ?? false,
    savedModels: normalizeModels(config?.models ?? []),
    items: [],
  };
}

export function resetModelsListState(
  target: ModelsListState,
  config?: Partial<ModelsListConfig> | null,
): void {
  const fresh = createModelsListState(config);
  target.enabled = fresh.enabled;
  target.savedModels = fresh.savedModels;
  target.items = fresh.items;
}

export function setModelsListCandidates(
  state: ModelsListState,
  candidates: string[],
  defaultSelected?: string[] | null,
): void {
  const normalizedCandidates = normalizeModels(candidates);
  const currentSelected = new Set(
    state.items.filter((item) => item.selected).map((item) => item.id),
  );
  const currentKnown = new Set(state.items.map((item) => item.id));
  const savedSelected = new Set(state.savedModels);
  const hasExistingItems = state.items.length > 0;
  const defaultSelectedSet =
    defaultSelected == null ? null : new Set(normalizeModels(defaultSelected));
  const selectionOrder = normalizeModels([
    ...state.items.map((item) => item.id),
    ...state.savedModels,
    ...normalizedCandidates,
  ]);

  state.items = selectionOrder.map((id) => {
    const selected = hasExistingItems
      ? currentSelected.has(id)
      : state.savedModels.length > 0
        ? savedSelected.has(id)
        : defaultSelectedSet
          ? defaultSelectedSet.has(id)
          : normalizedCandidates.includes(id);

    return {
      id,
      selected:
        selected &&
        (currentKnown.has(id) ||
          savedSelected.has(id) ||
          state.savedModels.length === 0),
    };
  });
}

export type ModelsListAddError = "empty" | "invalid" | "duplicate";

export function addCustomModelsListItems(
  state: ModelsListState,
  raw: string,
): ModelsListAddError | null {
  const parts = raw
    .split(/[\s,;，、]+/)
    .map((part) => part.trim())
    .filter(Boolean);
  if (parts.length === 0) {
    return "empty";
  }
  if (parts.some((part) => !isConcreteModelID(part))) {
    return "invalid";
  }

  let added = 0;
  const seen = new Set(state.items.map((item) => item.id));
  for (const part of parts) {
    const existing = state.items.find((item) => item.id === part);
    if (existing) {
      if (!existing.selected) {
        existing.selected = true;
        added += 1;
      }
      continue;
    }
    if (seen.has(part)) {
      continue;
    }
    seen.add(part);
    state.items.push({ id: part, selected: true });
    added += 1;
  }
  if (added > 0) {
    state.items = state.items.slice();
  }
  return added === 0 ? "duplicate" : null;
}

function isConcreteModelID(id: string): boolean {
  if (id.length === 0 || id.length > 128) {
    return false;
  }
  if (id.includes("*") || /\s/.test(id)) {
    return false;
  }
  return /[A-Za-z0-9]/.test(id);
}

export function buildModelsListConfig(state: ModelsListState): ModelsListConfig {
  return {
    enabled: state.enabled,
    models:
      state.items.length > 0
        ? state.items.filter((item) => item.selected).map((item) => item.id)
        : [...state.savedModels],
  };
}

export function selectAllModelsListItems(state: ModelsListState): void {
  state.items.forEach((item) => {
    item.selected = true;
  });
}

export function invertModelsListSelection(state: ModelsListState): void {
  state.items.forEach((item) => {
    item.selected = !item.selected;
  });
}

export function toggleModelsListItem(state: ModelsListState, modelID: string): void {
  const item = state.items.find((candidate) => candidate.id === modelID);
  if (item) {
    item.selected = !item.selected;
  }
}

export function moveModelsListItem(
  state: ModelsListState,
  fromIndex: number,
  toIndex: number,
): void {
  if (
    fromIndex === toIndex ||
    fromIndex < 0 ||
    toIndex < 0 ||
    fromIndex >= state.items.length ||
    toIndex >= state.items.length
  ) {
    return;
  }
  const [item] = state.items.splice(fromIndex, 1);
  state.items.splice(toIndex, 0, item);
}

export function normalizeModels(models: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of models) {
    const model = raw.trim();
    if (!model || seen.has(model)) {
      continue;
    }
    seen.add(model);
    out.push(model);
  }
  return out;
}
