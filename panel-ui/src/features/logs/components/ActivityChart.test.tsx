import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityChart } from "./ActivityChart";
import { chartLayers, savedStyle, saveStyle } from "./chartData";

const slots = ["2026-11-25T10:00:00Z", "2026-11-25T11:00:00Z"];
const b = (...n: number[]) => n.map((count, i) => ({ ts: slots[i] as string, count }));
const byType = { "request.blocked": b(2, 0), "field.rejected": b(1, 0), "connector.down": b(0, 1), "request.served": b(400, 50) };

describe("chartLayers", () => {
  it("sums each kind's types; served apart, only when selected", () => {
    const l = chartLayers(byType, slots, ["blocked", "field", "alert"]);
    expect(l.refused.map((x) => [x.kind.id, x.counts])).toEqual([["blocked", [2, 0]], ["field", [1, 0]], ["alert", [0, 1]]]);
    expect(l.served).toBeNull();
    expect(chartLayers(byType, slots, ["served"]).served).toEqual([400, 50]);
  });
});

describe("ActivityChart", () => {
  afterEach(cleanup);

  it("is one chart whose style the viewer picks; a bar picks its interval", () => {
    const onStyle = vi.fn();
    const onPick = vi.fn();
    const l = chartLayers(byType, slots, ["blocked", "field", "alert", "served"]);
    render(<ActivityChart slots={slots} refused={l.refused} served={l.served} step={3600} style="overlaid" onStyle={onStyle} onPick={onPick} />);
    expect(screen.getByText("4 refused")).toBeTruthy();
    expect(screen.getByText("450 served")).toBeTruthy();
    expect(screen.getByRole("radio", { name: "Overlaid" }).getAttribute("aria-checked")).toBe("true");
    fireEvent.click(screen.getByRole("radio", { name: "Stacked" }));
    expect(onStyle).toHaveBeenCalledWith("stacked");
    const bars = screen.getAllByRole("button", { name: /refused/ });
    expect(bars).toHaveLength(2);
    expect(bars[0]?.getAttribute("aria-label")).toMatch(/3 refused, 400 served$/);
    fireEvent.click(bars[1] as HTMLElement);
    expect(onPick).toHaveBeenCalledWith(slots[1]);
  });

  it("draws every style", () => {
    const l = chartLayers(byType, slots, ["blocked", "served"]);
    for (const style of ["overlaid", "side", "stacked"] as const) {
      const { container, unmount } = render(<ActivityChart slots={slots} refused={l.refused} served={l.served} step={3600} style={style} onStyle={() => {}} />);
      expect(container.querySelectorAll("button[aria-pressed] span").length).toBeGreaterThan(1);
      unmount();
    }
  });

  it("keeps the viewer's style in the browser, overlaid by default", () => {
    window.localStorage.clear();
    expect(savedStyle()).toBe("overlaid");
    saveStyle("side");
    expect(savedStyle()).toBe("side");
    window.localStorage.setItem("equalizer.logs.chart", "pie");
    expect(savedStyle()).toBe("overlaid");
  });
});
