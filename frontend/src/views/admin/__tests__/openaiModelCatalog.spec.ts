import { describe, expect, it } from "vitest";
import {
  addDisplayModel,
  isGrokModelID,
  moveModelID,
  normalizeModelIDs,
  removeWhitelistModel,
} from "../openaiModelCatalog";

describe("openaiModelCatalog helpers", () => {
  it("treats grok-4.5 as a Grok id", () => {
    expect(isGrokModelID("grok-4.5")).toBe(true);
    expect(isGrokModelID("gpt-6-astra")).toBe(false);
  });

  it("adds non-Grok display models to the whitelist", () => {
    const next = addDisplayModel(["gpt-5.6-sol"], ["gpt-5.6-sol"], "gpt-6-astra");
    expect(next.display).toEqual(["gpt-5.6-sol", "gpt-6-astra"]);
    expect(next.whitelist).toEqual(["gpt-5.6-sol", "gpt-6-astra"]);
  });

  it("does not auto-add Grok display models to the whitelist", () => {
    const next = addDisplayModel(["gpt-5.6-sol"], ["gpt-5.6-sol"], "grok-4.5");
    expect(next.display).toEqual(["gpt-5.6-sol", "grok-4.5"]);
    expect(next.whitelist).toEqual(["gpt-5.6-sol"]);
  });

  it("removing a non-Grok whitelist id also removes it from display", () => {
    const next = removeWhitelistModel(
      ["gpt-6-astra", "gpt-5.6-sol"],
      ["gpt-6-astra", "gpt-5.6-sol"],
      "gpt-6-astra",
    );
    expect(next.display).toEqual(["gpt-5.6-sol"]);
    expect(next.whitelist).toEqual(["gpt-5.6-sol"]);
  });

  it("keeps Grok on display when removing it from a manually added whitelist", () => {
    const next = removeWhitelistModel(
      ["grok-4.5"],
      ["grok-4.5"],
      "grok-4.5",
    );
    expect(next.display).toEqual(["grok-4.5"]);
    expect(next.whitelist).toEqual([]);
  });

  it("moves items and normalizes blanks", () => {
    expect(moveModelID(["a", "b", "c"], 2, 0)).toEqual(["c", "a", "b"]);
    expect(normalizeModelIDs([" a ", "", "a", "b"])).toEqual(["a", "b"]);
  });
});
