import { describe, expect, it } from "vitest";
import { byOwner, owner } from "./owner";

describe("owner", () => {
  it("names the plugin or theme of a file", () => {
    expect(owner("wp-content/plugins/akismet/views/enter.php")).toBe("plugin akismet");
    expect(owner("wp-content/themes/astra/functions.php")).toBe("theme astra");
    expect(owner("export.php")).toBe("the site's own code");
  });
  it("prefers the extension the reader named", () => {
    expect(owner("web/modules/contrib/webform/src/X.php", "module webform")).toBe("module webform");
    expect(owner("components/com_shop/shop.php", null)).toBe("the site's own code");
  });
  it("groups, the largest first", () => {
    const g = byOwner(["wp-content/plugins/a/x.php", "wp-content/plugins/b/y.php", "wp-content/plugins/b/z.php"], (f) => f);
    expect(g.map(([k, v]) => [k, v.length])).toEqual([["plugin b", 2], ["plugin a", 1]]);
  });
});
