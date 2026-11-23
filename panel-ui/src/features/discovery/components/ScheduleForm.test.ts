import { describe, expect, it } from "vitest";
import { inZone } from "./ScheduleForm";

describe("inZone", () => {
  it("shows a schedule saved in UTC at its real hour in Rome", () => {
    // 03:00 UTC in November is 04:00 in Rome (UTC+1): the panel must say 04
    expect(inZone("2026-11-24T03:00:00Z", "Europe/Rome")).toEqual({ hour: 4, weekday: 2 });
    expect(inZone("2026-07-14T01:00:00Z", "Europe/Rome")).toEqual({ hour: 3, weekday: 2 });
  });
});
