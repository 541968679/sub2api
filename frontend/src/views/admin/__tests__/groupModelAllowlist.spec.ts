import { describe, expect, it } from "vitest";
import {
  addCustomModelAllowlistItem,
  buildModelAllowlistConfig,
  createModelAllowlistState,
  setModelAllowlistCandidates,
} from "../groupModelAllowlist";

describe("groupModelAllowlist", () => {
  it("defaults to disabled with no selected models", () => {
    const state = createModelAllowlistState();
    expect(state.enabled).toBe(false);
    expect(buildModelAllowlistConfig(state)).toEqual({
      enabled: false,
      models: [],
    });
  });

  it("accepts trailing wildcard custom entries and rejects mid-string wildcards", () => {
    const state = createModelAllowlistState({ enabled: true });
    expect(addCustomModelAllowlistItem(state, "gpt-5.5-*")).toBeNull();
    expect(addCustomModelAllowlistItem(state, "gpt-*-5.4")).toBe("invalid_wildcard");
    expect(buildModelAllowlistConfig(state).models).toEqual(["gpt-5.5-*"]);
  });

  it("keeps saved allowlist entries when candidates load", () => {
    const state = createModelAllowlistState({
      enabled: true,
      models: ["gpt-5.5-*", "gpt-5.4"],
    });
    setModelAllowlistCandidates(state, ["gpt-5.4", "gpt-5.5-codex"]);
    expect(buildModelAllowlistConfig(state).models).toEqual(["gpt-5.5-*", "gpt-5.4"]);
  });
});
