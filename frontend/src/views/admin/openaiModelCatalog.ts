export function isGrokModelID(id: string): boolean {
  const model = id.trim().toLowerCase();
  return model === "grok" || model.startsWith("grok-");
}

export function normalizeModelIDs(ids: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of ids) {
    const id = raw.trim();
    if (!id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    out.push(id);
  }
  return out;
}

export function addDisplayModel(
  display: string[],
  whitelist: string[],
  modelID: string,
  platform = "openai",
): { display: string[]; whitelist: string[] } {
  const id = modelID.trim();
  if (!id) {
    return { display: normalizeModelIDs(display), whitelist: normalizeModelIDs(whitelist) };
  }
  const nextDisplay = normalizeModelIDs([...display, id]);
  const nextWhitelist =
    platform === "openai" && isGrokModelID(id)
      ? normalizeModelIDs(whitelist)
      : normalizeModelIDs([...whitelist, id]);
  return { display: nextDisplay, whitelist: nextWhitelist };
}

export function removeWhitelistModel(
  display: string[],
  whitelist: string[],
  modelID: string,
  platform = "openai",
): { display: string[]; whitelist: string[] } {
  const id = modelID.trim();
  const nextWhitelist = normalizeModelIDs(whitelist).filter((item) => item !== id);
  const nextDisplay =
    platform === "openai" && isGrokModelID(id)
      ? normalizeModelIDs(display)
      : normalizeModelIDs(display).filter((item) => item !== id);
  return { display: nextDisplay, whitelist: nextWhitelist };
}

export function moveModelID(ids: string[], fromIndex: number, toIndex: number): string[] {
  if (
    fromIndex === toIndex ||
    fromIndex < 0 ||
    toIndex < 0 ||
    fromIndex >= ids.length ||
    toIndex >= ids.length
  ) {
    return ids;
  }
  const next = [...ids];
  const [item] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, item);
  return next;
}
