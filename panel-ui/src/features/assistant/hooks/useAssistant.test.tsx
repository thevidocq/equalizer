import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { useAssistant } from "./useAssistant";

const sent = vi.hoisted(() => [] as { text: string; conversation?: string }[]);
const gate = vi.hoisted(() => ({ wait: Promise.resolve() as Promise<void> }));
vi.mock("@/shared/api/client", () => ({
  api: {
    POST: vi.fn(async (_path: string, opts: { body: { text: string; conversation?: string } }) => {
      sent.push(opts.body);
      await gate.wait;
      return { data: { conversation: "c1", text: "answer", proposals: [], cards: [], used: [] }, response: { ok: true } };
    }),
  },
  unwrap: async (p: Promise<{ data: unknown }>) => (await p).data,
  ApiError: class extends Error {},
}));

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

describe("useAssistant", () => {
  it("sends each question once, then goes on in the same conversation", async () => {
    const { result } = renderHook(() => useAssistant("blog"), { wrapper });
    act(() => result.current.send("first"));
    await waitFor(() => expect(result.current.messages).toHaveLength(2));
    act(() => result.current.send("second"));
    await waitFor(() => expect(result.current.messages).toHaveLength(4));
    expect(sent).toEqual([{ text: "first" }, { text: "second", conversation: "c1" }]);
    act(() => result.current.startNew());
    act(() => result.current.send("third"));
    await waitFor(() => expect(sent).toHaveLength(3));
    expect(sent[2]).toEqual({ text: "third" });
  });
  it("drops an answer that arrives after the person moved on", async () => {
    let release = () => {};
    gate.wait = new Promise<void>((r) => { release = r; });
    const { result } = renderHook(() => useAssistant("blog"), { wrapper });
    act(() => result.current.send("slow"));
    act(() => result.current.startNew());
    await act(async () => { release(); await gate.wait; });
    await waitFor(() => expect(result.current.busy).toBe(false));
    expect(result.current.messages).toEqual([]);
    expect(result.current.conversation).toBeNull();
    gate.wait = Promise.resolve();
  });
});
