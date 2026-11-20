/**
 * Login and admin paths of each CMS, with the side doors that take a
 * password without the login page (WordPress XML-RPC).
 */
export const loginPresets: Record<string, string[]> = {
  wordpress: ["/wp-admin/", "/wp-login.php", "/xmlrpc.php"],
  joomla: ["/administrator/"],
  drupal: ["/user/login", "/user/password", "/admin/"],
  moodle: ["/login/", "/admin/"],
  magento: ["/customer/account/login/", "/admin/"],
  opencart: ["/admin/"],
  typo3: ["/typo3/"],
  phpbb: ["/ucp.php", "/adm/"],
};

export const presetNames: Record<string, string> = {
  wordpress: "WordPress", joomla: "Joomla", drupal: "Drupal", moodle: "Moodle", magento: "Magento", opencart: "OpenCart",
  typo3: "TYPO3", phpbb: "phpBB",
};
