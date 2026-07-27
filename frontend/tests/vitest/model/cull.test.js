import { describe, it, expect, vi } from "vitest";
import "../fixtures";
import Cull from "model/cull";

describe("model/cull", () => {
  it("getDefaults includes cull fields", () => {
    const cull = new Cull();
    expect(cull.UID).toBe("");
    expect(cull.KeeperUID).toBe("");
    expect(cull.MemberCount).toBe(0);
    expect(cull.Thumb).toBe("");
    expect(cull.Members).toEqual([]);
  });

  it("getCollectionResource is culls", () => {
    expect(Cull.getCollectionResource()).toBe("culls");
  });

  it("classes includes cull uid", () => {
    const cull = new Cull({ UID: "ns6sg6be2lvl0yh7" });
    expect(cull.classes()).toContain("is-cull");
    expect(cull.classes()).toContain("uid-ns6sg6be2lvl0yh7");
  });

  it("setKeeper posts to keeper endpoint", async () => {
    const cull = new Cull({ UID: "ns6sg6be2lvl0yh7" });
    const post = vi.spyOn(await import("common/api").then((m) => m.default), "post").mockResolvedValue({
      data: { UID: "ns6sg6be2lvl0yh7", KeeperUID: "ps6sg6be2lvl0yh7" },
    });

    await cull.setKeeper("ps6sg6be2lvl0yh7");
    expect(post).toHaveBeenCalledWith("culls/ns6sg6be2lvl0yh7/keeper", { PhotoUID: "ps6sg6be2lvl0yh7" });
    post.mockRestore();
  });
});
