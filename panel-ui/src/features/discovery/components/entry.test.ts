import { describe, expect, it } from "vitest";
import { entryLine } from "./entry";

describe("entryLine", () => {
  it("writes the query that selects the entry point", () => {
    expect(entryLine({ path: "/index.php", origin: "opencart_route", query: { route: "extension/acme/module/wishlist.add" }, methods: ["GET", "POST"], public: true }))
      .toBe("/index.php?route=extension/acme/module/wishlist.add  GET,POST  route, no login");
  });
  it("reads a report written before the query", () => {
    expect(entryLine({ path: "/wp-admin/admin-ajax.php", origin: "wp_ajax", action: "save" })).toBe("/wp-admin/admin-ajax.php?action=save  AJAX (logged in)");
  });
});
