import type { Schemas } from "@/shared/api/client";

type Unseen = NonNullable<Schemas["StudySource"]["unseen"]>[number];

/** What each kind of entry point is, in a few words. */
export const ORIGIN: Record<string, string> = {
  rest_route: "REST", wp_ajax: "AJAX (logged in)", wp_ajax_nopriv: "AJAX (anyone)", admin_post: "admin-post", php_file: "PHP file",
  drupal_route: "Drupal route", joomla_component: "component", joomla_api: "web service", magento_route: "controller",
  magento_webapi: "REST", prestashop_controller: "module controller", opencart_route: "route", typo3_eid: "eID",
  typo3_backend: "backend route", mediawiki_special: "special page", mediawiki_api: "API module", mediawiki_rest: "REST",
  phpbb_route: "extension page", moodle_service: "AJAX service",
};

/** One entry point on a line: path and query, methods, kind, login. */
export function entryLine(u: Unseen): string {
  const query = u.query ?? (u.action ? { action: u.action } : null);
  const q = query ? "?" + Object.entries(query).map(([k, v]) => `${k}=${v}`).join("&") : "";
  const methods = u.methods && u.methods.length ? `${u.methods.join(",")}  ` : "";
  return `${u.path}${q}  ${methods}${ORIGIN[u.origin] ?? u.origin}${u.public ? ", no login" : ""}`;
}
