import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityChart } from "./ActivityChart";

const at = (h: number, count: number) => ({ ts: `2026-11-25T${String(h).padStart(2, "0")}:00:00Z`, count });

describe("ActivityChart", () => {
  afterEach(cleanup);

  it("is one chart: refused only while Served is not selected", () => {
    render(<ActivityChart refused={[at(10, 3), at(11, 0)]} served={null} step={3600} />);
    expect(screen.getByText("3 refused")).toBeTruthy();
    expect(screen.queryByText(/served/)).toBeNull();
    expect(screen.getAllByRole("button")).toHaveLength(2);
  });

  it("adds the served behind the refused, in the same bars; a bar picks its interval", () => {
    const onPick = vi.fn();
    render(<ActivityChart refused={[at(10, 3), at(11, 0)]} served={[at(10, 400), at(11, 50)]} step={3600} onPick={onPick} />);
    expect(screen.getByText("450 served")).toBeTruthy();
    const bars = screen.getAllByRole("button");
    expect(bars).toHaveLength(2);
    expect((bars[1] as HTMLButtonElement).disabled).toBe(false); // served only: still something to show
    fireEvent.click(bars[1] as HTMLElement);
    expect(onPick).toHaveBeenCalledWith(at(11, 0));
    expect(bars[0]?.getAttribute("aria-label")).toMatch(/3 refused, 400 served$/);
  });
});
