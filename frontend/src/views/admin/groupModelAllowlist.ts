export interface ModelAllowlistConfig {
  enabled: boolean
  models: string[]
}

export interface ModelAllowlistItem {
  id: string
  selected: boolean
}

export interface ModelAllowlistState {
  enabled: boolean
  savedModels: string[]
  items: ModelAllowlistItem[]
}

export type ModelAllowlistAddError = "empty" | "invalid_wildcard" | "duplicate"

export function createModelAllowlistState(
  config?: Partial<ModelAllowlistConfig> | null,
): ModelAllowlistState {
  return {
    enabled: config?.enabled ?? false,
    savedModels: normalizeModels(config?.models ?? []),
    items: [],
  }
}

export function resetModelAllowlistState(
  target: ModelAllowlistState,
  config?: Partial<ModelAllowlistConfig> | null,
): void {
  const fresh = createModelAllowlistState(config)
  target.enabled = fresh.enabled
  target.savedModels = fresh.savedModels
  target.items = fresh.items
}

export function setModelAllowlistCandidates(
  state: ModelAllowlistState,
  candidates: string[],
  defaultSelected?: string[] | null,
): void {
  const normalizedCandidates = normalizeModels(candidates)
  const currentSelected = new Set(
    state.items.filter((item) => item.selected).map((item) => item.id),
  )
  const currentKnown = new Set(state.items.map((item) => item.id))
  const savedSelected = new Set(state.savedModels)
  const hasExistingItems = state.items.length > 0
  const defaultSelectedSet =
    defaultSelected == null ? null : new Set(normalizeModels(defaultSelected))
  const selectionOrder = normalizeModels([
    ...state.items.map((item) => item.id),
    ...state.savedModels,
    ...normalizedCandidates,
  ])

  state.items = selectionOrder.map((id) => {
    const selected = hasExistingItems
      ? currentSelected.has(id)
      : state.savedModels.length > 0
        ? savedSelected.has(id)
        : defaultSelectedSet
          ? defaultSelectedSet.has(id)
          : normalizedCandidates.includes(id)

    return {
      id,
      selected:
        selected &&
        (currentKnown.has(id) ||
          savedSelected.has(id) ||
          state.savedModels.length === 0),
    }
  })
}

export function toggleModelAllowlistItem(
  state: ModelAllowlistState,
  modelID: string,
): void {
  const item = state.items.find((candidate) => candidate.id === modelID)
  if (item) {
    item.selected = !item.selected
  }
}

export function selectAllModelAllowlistItems(state: ModelAllowlistState): void {
  state.items.forEach((item) => {
    item.selected = true
  })
}

export function invertModelAllowlistSelection(state: ModelAllowlistState): void {
  state.items.forEach((item) => {
    item.selected = !item.selected
  })
}

export function moveModelAllowlistItem(
  state: ModelAllowlistState,
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
    return
  }
  const [item] = state.items.splice(fromIndex, 1)
  state.items.splice(toIndex, 0, item)
}

export function addCustomModelAllowlistItem(
  state: ModelAllowlistState,
  raw: string,
): ModelAllowlistAddError | null {
  const entry = raw.trim()
  if (!entry) {
    return "empty"
  }
  if (entry.slice(0, -1).includes("*")) {
    return "invalid_wildcard"
  }
  if (
    state.items.some((item) => item.id.toLowerCase() === entry.toLowerCase()) ||
    state.savedModels.some((model) => model.toLowerCase() === entry.toLowerCase())
  ) {
    return "duplicate"
  }
  state.items.push({ id: entry, selected: true })
  return null
}

export function buildModelAllowlistConfig(
  state: ModelAllowlistState,
): ModelAllowlistConfig {
  return {
    enabled: state.enabled,
    models:
      state.items.length > 0
        ? state.items.filter((item) => item.selected).map((item) => item.id)
        : [...state.savedModels],
  }
}

function normalizeModels(models: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of models) {
    const model = raw.trim()
    if (!model || seen.has(model)) {
      continue
    }
    seen.add(model)
    out.push(model)
  }
  return out
}
