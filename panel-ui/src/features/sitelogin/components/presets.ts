/**
 * Login and admin paths of each CMS, with the side doors that take a
 * password without the login page (WordPress XML-RPC).
 */
export const loginPresets: Record<string, string[]> = {
  wordpress: ["/wp-admin/", "/wp-login.php", "/xmlrpc.php"],
  joomla: ["/administrator/"],
  drupal: ["/user/login", "/user/password", "/admin/"],
  moodle: ["/login/", "/admin/"],
};

export const presetNames: Record<string, string> = {
  wordpress: "WordPress", joomla: "Joomla", drupal: "Drupal", moodle: "Moodle",
};
