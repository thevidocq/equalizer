import { describe, expect, it } from "vitest";
import { history, type Message } from "./history";

describe("history", () => {
  it("sends answered turns only, the new question last", () => {
    const m: Message[] = [
      { role: "user", text: "a" }, { role: "assistant", text: "b" },
      { role: "user", text: "c", failed: true }, { role: "assistant", text: "provider down", failed: true },
    ];
    expect(history(m, "d")).toEqual([{ role: "user", text: "a" }, { role: "assistant", text: "b" }, { role: "user", text: "d" }]);
  });
  it("keeps at most 20 messages, starting with a question", () => {
    const m: Message[] = Array.from({ length: 30 }, (_, i) => ({ role: i % 2 ? "assistant" : "user", text: String(i) }));
    const h = history(m, "new");
    expect(h.length).toBeLessThanOrEqual(20);
    expect(h[0]?.role).toBe("user");
    expect(h.at(-1)).toEqual({ role: "user", text: "new" });
  });
  it("cuts a long answer to what the API takes", () => {
    const h = history([{ role: "user", text: "q" }, { role: "assistant", text: "x".repeat(5000) }], "next");
    expect(Array.from(h[1]?.text ?? "").length).toBe(4000);
  });
});
