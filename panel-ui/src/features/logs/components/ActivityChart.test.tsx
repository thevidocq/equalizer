import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityChart } from "./ActivityChart";

const at = (h: number, count: number) => ({ ts: `2026-11-25T${String(h).padStart(2, "0")}:00:00Z`, count });

describe("ActivityChart", () => {
  afterEach(cleanup);

  it("draws served and refused bars on the same hours; a bar picks its interval and kind", () => {
    const onPick = vi.fn();
    render(<ActivityChart served={[at(10, 400), at(11, 0)]} refused={[at(10, 3), at(11, 7)]} step={3600} onPick={onPick} />);
    expect(screen.getByText("400 in the period")).toBeTruthy();
    expect(screen.getByText("10 in the period")).toBeTruthy();
    const served = screen.getAllByRole("button", { name: /served$/ });
    expect(served).toHaveLength(2);
    expect((served[1] as HTMLButtonElement).disabled).toBe(true); // nothing to show
    fireEvent.click(served[0] as HTMLElement);
    expect(onPick).toHaveBeenCalledWith(at(10, 400), "served");
    fireEvent.click(screen.getAllByRole("button", { name: /refused$/ })[1] as HTMLElement);
    expect(onPick).toHaveBeenLastCalledWith(at(11, 7), "refused");
  });

  it("marks the picked bar of its own row only", () => {
    render(<ActivityChart served={[at(10, 4)]} refused={[at(10, 3)]} step={3600} picked={{ ts: at(10, 0).ts, series: "served" }} />);
    expect(screen.getByRole("button", { name: /served$/ }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: /refused$/ }).getAttribute("aria-pressed")).toBe("false");
  });
});
