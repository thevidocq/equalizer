/** Why something is still open (StillOpen.reason), in words. */
export const STILL_OPEN: Record<string, string> = {
  not_reached: "not opened, served by the site area", legacy_error: "the site answered an error",
  action_not_performed: "changes something: never opened",
  browser_skipped: "its scripts not studied: skipped in the browser, served by the site area", cannot_narrow: "values could not be narrowed",
  accepts_sql: "accepts SQL characters", accepts_markup: "accepts HTML", accepts_template: "accepts template code",
  accepts_path: "accepts paths", accepts_shell: "accepts shell characters", other: "other",
};
