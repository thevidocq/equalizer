package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// tree writes a site of a few files and reads it. Its summary is the
// application's example in schema/source/examples (the panel checks each
// against the schema): -update rewrites them.
func tree(t *testing.T, files map[string]string) *Summary {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Read(dir, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	example(t, s)
	return s
}

func example(t *testing.T, s *Summary) {
	t.Helper()
	name := strings.ToLower(strings.TrimPrefix(t.Name(), "Test"))
	b, _ := json.MarshalIndent(s, "", "  ")
	b = append(b, '\n')
	p := "../../../schema/source/examples/" + name + ".json"
	if *update {
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if have, err := os.ReadFile(p); err != nil || string(have) != string(b) {
		t.Errorf("the summary differs from %s (go test ./internal/extract -update to rewrite it)", p)
	}
}

// routes lists a summary's routes as "path?k=v ORIGIN public".
func routes(s *Summary) []string {
	var out []string
	for _, r := range s.Routes {
		p := r.Path
		var q []string
		for _, k := range sortedKeys(r.Query) {
			q = append(q, k+"="+r.Query[k])
		}
		if len(q) > 0 {
			p += "?" + strings.Join(q, "&")
		}
		p += " " + strings.Join(r.Methods, ",") + " " + r.Origin
		if r.Public {
			p += " public"
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func expect(t *testing.T, s *Summary, stack, version string, want ...string) {
	t.Helper()
	if s.Stack != stack {
		t.Fatalf("stack %q, want %q", s.Stack, stack)
	}
	got := ""
	for _, c := range s.Components {
		if c.Kind == "core" && c.Version != nil {
			got = *c.Version
		}
	}
	if got != version {
		t.Errorf("core version %q, want %q", got, version)
	}
	have := map[string]bool{}
	for _, r := range routes(s) {
		have[r] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing %q in\n%s", w, strings.Join(routes(s), "\n"))
		}
	}
	for _, r := range s.Routes {
		if strings.Contains(r.Where.File, "core") && r.Origin != "php_file" {
			t.Errorf("a route of the core: %+v", r)
		}
	}
}

const acting = "<?php echo $_GET['q'];"

func TestDrupal(t *testing.T) {
	s := tree(t, map[string]string{
		"web/core/lib/Drupal.php":                      "<?php class Drupal { const VERSION = '10.3.1'; }",
		"web/core/modules/node/node.routing.yml":       "node.add:\n  path: '/node/add'\n",
		"web/core/install.php":                         acting,
		"web/index.php":                                acting,
		"web/modules/contrib/webform/webform.info.yml": "name: Webform\nversion: '6.2.3'\n",
		"web/modules/contrib/webform/webform.routing.yml": `webform.share:
  path: '/webform/{webform}/share'
  defaults:
    _controller: '\Drupal\webform\Controller::share'
  methods: [GET]
  requirements:
    _access: 'TRUE'
    webform: '[a-z0-9_]+'

webform.admin:
  path: '/admin/structure/webform/{id}'
  requirements:
    _permission: 'administer webform'
    id: \d+
`,
		"composer.lock":    `{"packages":[{"name":"drupal/core","version":"10.3.1"}]}`,
		"scripts/tool.php": acting, // outside web/: no browser opens it
	})
	expect(t, s, "drupal", "10.3.1",
		"/webform/{webform}/share GET drupal_route public",
		"/admin/structure/webform/{id} GET,POST drupal_route")
	for _, r := range s.Routes {
		if r.Path == "/admin/structure/webform/{id}" && r.Params["id"] != "int" || r.Origin == "php_file" {
			t.Errorf("%+v", r)
		}
		if r.Where.Owner != "module webform" {
			t.Errorf("owner %q", r.Where.Owner)
		}
	}
	if s.Files.Known < 3 {
		t.Errorf("the core is read: %+v", s.Files)
	}
}

func TestDrupal7(t *testing.T) {
	s := tree(t, map[string]string{
		"includes/bootstrap.inc":               "<?php define('VERSION', '7.101');",
		"includes/menu.inc":                    "<?php",
		"modules/system/system.module":         "<?php $items['admin'] = array();",
		"sites/all/modules/mymod/mymod.module": "<?php function mymod_menu() {\n$items['mymod/%node/view'] = array(\n 'access callback' => TRUE,\n);\n$items['admin/mymod'] = array(\n 'access arguments' => array('x'),\n);\nreturn $items; }",
	})
	expect(t, s, "drupal", "7.101", "/mymod/{node}/view GET,POST drupal_route public", "/admin/mymod GET,POST drupal_route")
	if len(s.Routes) != 2 {
		t.Errorf("%v", routes(s))
	}
}

func TestJoomla(t *testing.T) {
	s := tree(t, map[string]string{
		"libraries/src/Version.php":                   "<?php final class Version { public const MAJOR_VERSION = 5;\n public const MINOR_VERSION = 1;\n public const PATCH_VERSION = 4; }",
		"libraries/src/Extension/ExtensionHelper.php": "<?php $x = [['component', 'com_content', '', 1], ['plugin', 'content', 'webservices', 0]];",
		"administrator/index.php":                     acting,
		"components/com_content/src/View.php":         acting,
		"plugins/webservices/content/content.php":     "<?php $router->createCRUDRoutes('v1/content/articles', 'articles', ['component' => 'com_content']);",
		"components/com_shop/shop.php":                "<?php \\defined('_JEXEC') or die; $id = $this->input->getInt('id'); $t = $app->getInput()->getCmd('task');",
		"components/com_shop/ajax.php":                acting,
		"administrator/components/com_shop/shop.php":  "<?php defined('_JEXEC') or die;",
		"plugins/webservices/shop/shop.php": "<?php\n$router->createCRUDRoutes(\n  'v1/shop/items',\n  'items',\n  ['component' => 'com_shop'],\n  true\n);\n" +
			"$router->addRoute(new Route(['POST'], 'v1/shop/cart/:id', 'cart.add', ['id' => '(\\d+)'], ['public' => true]));",
	})
	expect(t, s, "joomla", "5.1.4",
		"/index.php?option=com_shop GET,POST joomla_component public",
		"/administrator/index.php?option=com_shop GET,POST joomla_component",
		"/api/index.php/v1/shop/items GET,POST joomla_api public",
		"/api/index.php/v1/shop/items/{id} GET,PATCH,DELETE joomla_api public",
		"/api/index.php/v1/shop/cart/{id} POST joomla_api public",
		"/components/com_shop/ajax.php GET,POST php_file public")
	types := map[string]string{}
	for _, in := range s.Inputs {
		types[in.Name] = in.Type
	}
	if types["id"] != "int" || types["task"] != "key" {
		t.Errorf("inputs %v", types)
	}
	for _, r := range s.Routes {
		if strings.Contains(r.Path, "content") {
			t.Errorf("the core's web service: %s", r.Path)
		}
	}
}

func TestMagento(t *testing.T) {
	s := tree(t, map[string]string{
		"app/bootstrap.php":                                         "<?php",
		"app/etc/NonComposerComponentRegistration.php":              "<?php",
		"app/code/Magento/Catalog/etc/frontend/routes.xml":          `<router id="standard"><route id="catalog" frontName="catalog"/></router>`,
		"app/code/Magento/Catalog/Controller/Product/View.php":      "<?php",
		"app/code/Acme/Quote/composer.json":                         `{"name":"acme/quote","version":"1.4.0"}`,
		"app/code/Acme/Quote/etc/frontend/routes.xml":               `<router id="standard"><route id="quote" frontName="quote"><module name="Acme_Quote"/></route></router>`,
		"app/code/Acme/Quote/etc/adminhtml/routes.xml":              `<router id="admin"><route id="quote" frontName="acmequote"/></router>`,
		"app/code/Acme/Quote/Controller/Request/Send.php":           "<?php class Send {}",
		"app/code/Acme/Quote/Controller/Adminhtml/Request/Grid.php": "<?php class Grid {}",
		"app/code/Acme/Quote/etc/webapi.xml": `<routes>
    <route url="/V1/acme-quotes/:quoteId" method="GET">
        <service class="X" method="get"/>
        <resources><resource ref="anonymous"/></resources>
    </route>
    <route url="/V1/acme-quotes/:quoteId" method="DELETE">
        <resources><resource ref="Acme_Quote::manage"/></resources>
    </route>
</routes>`,
		"composer.lock": `{"packages":[{"name":"magento/product-community-edition","version":"2.4.7-p3"}]}`,
	})
	expect(t, s, "magento", "2.4.7-p3",
		"/quote/request/send GET,POST magento_route",
		"/admin/acmequote/request/grid GET,POST magento_route",
		"/rest/V1/acme-quotes/{quoteId} GET magento_webapi public",
		"/rest/V1/acme-quotes/{quoteId} DELETE magento_webapi")
	if len(s.Routes) != 4 {
		t.Errorf("%v", routes(s))
	}
}

func TestPrestaShop(t *testing.T) {
	s := tree(t, map[string]string{
		"classes/ObjectModel.php":                      acting,
		"config/defines.inc.php":                       "<?php",
		"src/Core/Version.php":                         "<?php class Version { public const VERSION = '8.1.7'; }",
		"composer.json":                                `{"require":{"prestashop/ps_banner":"^2"}}`,
		"admin123xyz/filemanager/x.php":                acting,
		"admin123xyz/index.php":                        acting,
		"modules/ps_banner/ps_banner.php":              acting,
		"modules/loyalty/loyalty.php":                  "<?php if (!defined('_PS_VERSION_')) { exit; } class Loyalty { function __construct() { $this->version = '3.2.1'; } }",
		"modules/loyalty/controllers/front/points.php": "<?php class LoyaltyPointsModuleFrontController { public $auth = true; }",
		"modules/loyalty/controllers/front/rules.php":  "<?php class LoyaltyRulesModuleFrontController {}",
		"modules/loyalty/ajax.php":                     acting,
	})
	expect(t, s, "prestashop", "8.1.7",
		"/index.php?controller=points&fc=module&module=loyalty GET,POST prestashop_controller",
		"/index.php?controller=rules&fc=module&module=loyalty GET,POST prestashop_controller public",
		"/modules/loyalty/ajax.php GET,POST php_file public")
	if len(s.Routes) != 3 {
		t.Errorf("%v", routes(s))
	}
}

func TestOpenCart(t *testing.T) {
	s := tree(t, map[string]string{
		"index.php":                          "<?php define('VERSION', '4.0.2.3');",
		"system/startup.php":                 "<?php",
		"catalog/controller/common/home.php": "<?php class Home { public function index() {} }",
		"extension/opencart/catalog/controller/payment/cod.php": "<?php class Cod { public function confirm() {} }",
		"extension/acme/install.json":                           `{"name":"Acme","version":"2.0.1"}`,
		"extension/acme/catalog/controller/module/wishlist.php": "<?php class Wishlist {\n public function index(): string {}\n public function add(): void {}\n private function helper() {}\n}",
		"extension/acme/admin/controller/module/wishlist.php":   "<?php class Wishlist { public function save(): void {} }",
	})
	expect(t, s, "opencart", "4.0.2.3",
		"/index.php?route=extension/acme/module/wishlist GET,POST opencart_route public",
		"/index.php?route=extension/acme/module/wishlist.add GET,POST opencart_route public",
		"/admin/index.php?route=extension/acme/module/wishlist.save GET,POST opencart_route")
	if len(s.Routes) != 3 {
		t.Errorf("%v", routes(s))
	}
}

func TestTYPO3(t *testing.T) {
	s := tree(t, map[string]string{
		"typo3/sysext/core/Classes/Information/Typo3Version.php":  "<?php final class Typo3Version { protected const VERSION = '12.4.20'; }",
		"typo3/sysext/frontend/ext_localconf.php":                 "<?php $GLOBALS['TYPO3_CONF_VARS']['FE']['eID_include']['tx_cms_showpic'] = 'x';",
		"typo3conf/ext/news/ext_emconf.php":                       "<?php $EM_CONF[$_EXTKEY] = ['version' => '11.4.1'];",
		"typo3conf/ext/news/ext_localconf.php":                    "<?php defined('TYPO3') or die();\n$GLOBALS['TYPO3_CONF_VARS']['FE']['eID_include']['news_feed'] = 'x';",
		"typo3conf/ext/news/Configuration/Backend/AjaxRoutes.php": "<?php return [\n 'news_import' => [\n  'path' => '/news/import',\n  'methods' => ['POST'],\n ],\n 'news_ping' => [\n  'path' => '/news/ping',\n  'access' => 'public',\n ],\n];",
	})
	expect(t, s, "typo3", "12.4.20",
		"/index.php?eID=news_feed GET,POST typo3_eid public",
		"/typo3/ajax/news/import POST typo3_backend",
		"/typo3/ajax/news/ping GET,POST typo3_backend public")
	if len(s.Routes) != 3 {
		t.Errorf("%v", routes(s))
	}
}

func TestTYPO3Composer(t *testing.T) {
	s := tree(t, map[string]string{
		"public/index.php":                "<?php call_user_func(function () { \\TYPO3\\CMS\\Core\\Core\\SystemEnvironmentBuilder::run(); });",
		"composer.lock":                   `{"packages":[{"name":"typo3/cms-core","version":"v13.4.2"}]}`,
		"packages/site/ext_localconf.php": "<?php defined('TYPO3') or die();\n$GLOBALS['TYPO3_CONF_VARS']['FE']['eID_include']['site_api'] = 'x';",
	})
	expect(t, s, "typo3", "13.4.2", "/index.php?eID=site_api GET,POST typo3_eid public")
}

func TestMediaWiki(t *testing.T) {
	s := tree(t, map[string]string{
		"includes/Defines.php": "<?php define( 'MW_VERSION', '1.42.3' );",
		"api.php":              acting,
		"extensions/Quiz/extension.json": `{"name":"Quiz","version":"1.2.0","SpecialPages":{"QuizResults":"X"},
			"APIModules":{"quizsubmit":"Y"},"APIListModules":{"quizzes":"Z"},
			"RestRoutes":[{"path":"/quiz/v0/{id}","method":["GET","POST"]},{"path":"/quiz/v0/list","method":"GET"}]}`,
		"extensions/Old/Old.php": "<?php if ( !defined( 'MEDIAWIKI' ) ) die();\n$wgSpecialPages['OldTool'] = 'OldTool';",
		"skins/Shiny/skin.json":  `{"name":"Shiny","version":"2.0"}`,
	})
	expect(t, s, "mediawiki", "1.42.3",
		"/index.php?title=Special:QuizResults GET,POST mediawiki_special",
		"/index.php?title=Special:OldTool GET,POST mediawiki_special",
		"/api.php?action=quizsubmit GET,POST mediawiki_api",
		"/api.php?action=query&list=quizzes GET,POST mediawiki_api",
		"/rest.php/quiz/v0/{id} GET,POST mediawiki_rest",
		"/rest.php/quiz/v0/list GET mediawiki_rest")
	names := map[string]bool{}
	for _, c := range s.Components {
		names[c.Name] = true
	}
	if !names["extension:Quiz"] || !names["skin:Shiny"] {
		t.Errorf("%+v", s.Components)
	}
}

func TestPhpBB(t *testing.T) {
	s := tree(t, map[string]string{
		"forum/includes/constants.php":              "<?php @define('PHPBB_VERSION', '3.3.14');",
		"forum/viewtopic.php":                       acting,
		"forum/config/default/routing/routing.yml":  "phpbb_index:\n    path: /index\n",
		"forum/ext/acme/gallery/composer.json":      `{"name":"acme/gallery","version":"1.1.0"}`,
		"forum/ext/acme/gallery/config/routing.yml": "acme_gallery_album:\n    path: /gallery/album/{album_id}\n    defaults: { _controller: acme.gallery.album:base }\n    requirements:\n        album_id: \\d+\n",
	})
	expect(t, s, "phpbb", "3.3.14", "/forum/app.php/gallery/album/{album_id} GET,POST phpbb_route")
	if len(s.Routes) != 1 || s.Routes[0].Params["album_id"] != "int" {
		t.Errorf("%v %+v", routes(s), s.Routes)
	}
}

func TestMoodle(t *testing.T) {
	s := tree(t, map[string]string{
		"lib/components.json":              `{"plugintypes":{"mod":"public/mod","local":"public/local"}}`,
		"lib/plugins.json":                 `{"standard":{"mod":["forum"],"local":[]}}`,
		"public/lib/moodlelib.php":         "<?php",
		"public/version.php":               "<?php $release = '5.1.1 (Build: 20251215)';",
		"public/mod/forum/version.php":     "<?php",
		"public/mod/forum/view.php":        "<?php require('../../config.php'); $id = optional_param('id', 0, PARAM_INT);",
		"public/local/reports/version.php": "<?php defined('MOODLE_INTERNAL') || die(); $plugin->release = '2.3';",
		"public/local/reports/index.php":   "<?php\nrequire_once(__DIR__ . '/../../config.php');\n$id = required_param('courseid', PARAM_INT);\n$q = optional_param('q', '', PARAM_TEXT);",
		"public/local/reports/lib.php":     "<?php defined('MOODLE_INTERNAL') || die(); function local_reports_x() {}",
		"public/local/reports/db/services.php": `<?php
$functions = [
    'local_reports_get' => [
        'classname' => 'x',
        'ajax' => true,
    ],
    'local_reports_public' => [
        'classname' => 'x',
        'ajax' => true,
        'loginrequired' => false,
    ],
    'local_reports_ws_only' => [
        'classname' => 'x',
    ],
];`,
	})
	expect(t, s, "moodle", "5.1.1",
		"/local/reports/index.php GET,POST php_file public",
		"/lib/ajax/service.php?info=local_reports_get POST moodle_service",
		"/lib/ajax/service-nologin.php?info=local_reports_public POST moodle_service public")
	if len(s.Routes) != 3 {
		t.Errorf("%v", routes(s))
	}
	types := map[string]string{}
	for _, in := range s.Inputs {
		types[in.Name] = in.Type + " " + in.Route
	}
	if types["courseid"] != "int /local/reports/index.php" || types["q"] != "text /local/reports/index.php" || types["id"] != "" {
		t.Errorf("inputs %v", types)
	}
}

func TestWordPressInAFolder(t *testing.T) {
	s := tree(t, map[string]string{
		"blog/wp-includes/version.php":          "<?php $wp_version = '6.7.1';",
		"blog/wp-admin/admin-ajax.php":          acting,
		"blog/wp-content/plugins/shop/shop.php": "<?php\n/*\nPlugin Name: Shop\nVersion: 1.0\n*/\nadd_action( 'wp_ajax_nopriv_shop_cart', 'f' );",
		"blog/custom.php":                       acting,
		"landing.php":                           acting,
	})
	expect(t, s, "wordpress", "6.7.1",
		"/blog/wp-admin/admin-ajax.php?action=shop_cart GET,POST wp_ajax_nopriv public",
		"/blog/custom.php GET,POST php_file public",
		"/landing.php GET,POST php_file public")
}
