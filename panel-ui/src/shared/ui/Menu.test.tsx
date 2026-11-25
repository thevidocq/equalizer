import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { Menu } from "./Menu";

afterEach(cleanup);

describe("Menu", () => {
  it("opens, moves with the arrows, chooses with Enter and closes", () => {
    const a = vi.fn(), b = vi.fn();
    render(<Menu label="Download" items={[{ label: "YAML", onSelect: a }, { label: "JSON", onSelect: b }]} />);
    const button = screen.getByRole("button", { name: "Download" });
    expect(screen.queryByRole("menu")).toBeNull();
    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    const items = screen.getAllByRole("menuitem");
    expect(document.activeElement).toBe(items[0]);
    fireEvent.keyDown(items[0]!, { key: "ArrowDown" });
    expect(document.activeElement).toBe(screen.getAllByRole("menuitem")[1]);
    fireEvent.click(screen.getAllByRole("menuitem")[1]!);
    expect(b).toHaveBeenCalledOnce();
    expect(a).not.toHaveBeenCalled();
    expect(screen.queryByRole("menu")).toBeNull();
  });
  it("closes with Escape", () => {
    render(<Menu label="Download" items={[{ label: "YAML", onSelect: () => {} }]} />);
    fireEvent.click(screen.getByRole("button", { name: "Download" }));
    fireEvent.keyDown(screen.getByRole("menuitem"), { key: "Escape" });
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
