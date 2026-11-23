import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { RichText } from "./RichText";

describe("RichText", () => {
  it("shows paragraphs, bullets and code, never HTML", () => {
    const { container } = render(<RichText text={"It was `?orderby=<script>`.\n\n- one\n- two"} />);
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("code")?.textContent).toBe("?orderby=<script>");
    expect(container.querySelectorAll("li")).toHaveLength(2);
  });
});
