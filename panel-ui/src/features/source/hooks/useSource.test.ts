import { describe, expect, it } from "vitest";
import { reading, type SourceReader } from "./useSource";

const reader = (status?: "queued" | "running" | "done" | "failed"): SourceReader => ({
  configured: true, status: "ready",
  last_read: status ? { id: 1, status, created_at: "2026-11-24T10:00:00Z" } : null,
});

describe("reading", () => {
  it("is true while a read waits or goes on", () => {
    expect(reading(reader("queued"))).toBe(true);
    expect(reading(reader("running"))).toBe(true);
  });
  it("is false after a read, or without one", () => {
    expect(reading(reader("done"))).toBe(false);
    expect(reading(reader("failed"))).toBe(false);
    expect(reading(reader())).toBe(false);
    expect(reading(undefined)).toBe(false);
  });
});
