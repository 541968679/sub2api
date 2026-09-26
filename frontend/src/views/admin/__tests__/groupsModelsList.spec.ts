import { describe, expect, it } from "vitest";
import {
  addCustomModelsListItems,
  buildModelsListConfig,
  createModelsListState,
  invertModelsListSelection,
  moveModelsListItem,
  normalizeModels,
  selectAllModelsListItems,
  setModelsListCandidates,
  toggleModelsListItem,
} from "../groupsModelsList";

describe("groupsModelsList", () => {
  it("defaults to disabled with no selected models", () => {
    const state = createModelsListState();

    expect(state.enabled).toBe(false);
    expect(buildModelsListConfig(state)).toEqual({
      enabled: false,
      models: [],
    });
  });

  it("hydrates saved models and preserves selection when candidates load", () => {
    const state = createModelsListState({
      enabled: true,
      models: ["gpt-5.5", "claude-sonnet-4-5"],
    });

    setModelsListCandidates(state, [
      "gpt-5.5",
      "gpt-5.4",
      "claude-sonnet-4-5",
    ]);

    expect(state.items).toEqual([
      { id: "gpt-5.5", selected: true },
      { id: "claude-sonnet-4-5", selected: true },
      { id: "gpt-5.4", selected: false },
    ]);
  });

  it("selects all candidates by default when there is no saved config", () => {
    const state = createModelsListState();

    setModelsListCandidates(state, ["gpt-5.5", "gpt-5.4"]);

    expect(buildModelsListConfig(state).models).toEqual(["gpt-5.5", "gpt-5.4"]);
  });

  it("supports toggle, invert, select all, and move", () => {
    const state = createModelsListState({ enabled: true });
    setModelsListCandidates(state, ["a", "b", "c"]);

    toggleModelsListItem(state, "b");
    expect(buildModelsListConfig(state).models).toEqual(["a", "c"]);

    invertModelsListSelection(state);
    expect(buildModelsListConfig(state).models).toEqual(["b"]);

    selectAllModelsListItems(state);
    moveModelsListItem(state, 2, 0);

    expect(buildModelsListConfig(state)).toEqual({
      enabled: true,
      models: ["c", "a", "b"],
    });
  });

  it("normalizes blank and duplicate model ids without lowercasing", () => {
    expect(normalizeModels([" gpt-5.5 ", "", "GPT-5.5", "gpt-5.5"])).toEqual([
      "gpt-5.5",
      "GPT-5.5",
    ]);
  });

  it("checks only the curated subset when a wider candidate list is provided", () => {
    const state = createModelsListState({ enabled: true });

    setModelsListCandidates(
      state,
      ["gpt-5.5", "gpt-image-2", "glm-4-flash"],
      ["gpt-5.5"],
    );

    expect(state.items).toEqual([
      { id: "gpt-5.5", selected: true },
      { id: "gpt-image-2", selected: false },
      { id: "glm-4-flash", selected: false },
    ]);
  });

  it("adds concrete model ids and selects an existing unchecked one", () => {
    const state = createModelsListState({ enabled: true });
    setModelsListCandidates(state, ["gpt-5.5", "gpt-image-2"], ["gpt-5.5"]);

    expect(addCustomModelsListItems(state, "my-model, gpt-image-2")).toBeNull();
    expect(buildModelsListConfig(state).models).toEqual([
      "gpt-5.5",
      "gpt-image-2",
      "my-model",
    ]);
    expect(addCustomModelsListItems(state, "gpt-*")).toBe("invalid");
    expect(addCustomModelsListItems(state, "my-model")).toBe("duplicate");
    expect(addCustomModelsListItems(state, "   ")).toBe("empty");
  });
});
