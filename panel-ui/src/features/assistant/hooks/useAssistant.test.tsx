import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { useAssistant } from "./useAssistant";

const sent = vi.hoisted(() => [] as { role: string; text: string }[][]);
vi.mock("@/shared/api/client", () => ({
  api: {
    POST: vi.fn(async (_path: string, opts: { body: { messages: { role: string; text: string }[] } }) => {
      sent.push(opts.body.messages);
      return { data: { text: "answer", proposals: [], cards: [], used: [] } };
    }),
  },
  unwrap: async (p: Promise<{ data: unknown }>) => (await p).data,
}));

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

describe("useAssistant", () => {
  it("sends each question once, after the answered turns", async () => {
    const { result } = renderHook(() => useAssistant("blog"), { wrapper });
    act(() => result.current.send("first"));
    await waitFor(() => expect(result.current.messages).toHaveLength(2));
    act(() => result.current.send("second"));
    await waitFor(() => expect(result.current.messages).toHaveLength(4));
    expect(sent[0]).toEqual([{ role: "user", text: "first" }]);
    expect(sent[1]).toEqual([
      { role: "user", text: "first" }, { role: "assistant", text: "answer" }, { role: "user", text: "second" },
    ]);
  });
});
