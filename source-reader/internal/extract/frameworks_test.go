package extract

import (
	"sort"
	"strings"
	"testing"
)

// inputs: "name source type" of the summary's inputs, sorted.
func inputs(s *Summary) []string {
	var out []string
	for _, i := range s.Inputs {
		out = append(out, i.Name+" "+i.Source+" "+i.Type)
	}
	sort.Strings(out)
	return out
}

func sameList(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// Every application's own request reader gives its keys and their types,
// as $_GET does on WordPress: the same depth on every kind of site.
func TestRequestReadersOfEveryApplication(t *testing.T) {
	t.Run("drupal", func(t *testing.T) {
		s := read(t, map[string]string{
			"web/core/lib/Drupal.php": "<?php class Drupal { const VERSION = '11.1.0'; }",
			"web/modules/contrib/shop/src/Controller/Cart.php": "<?php\n$id = $request->query->getInt('product');\n" +
				"$q = $this->request->query->get('q');\n$email = $request->request->get('email');\n" +
				"$n = (int) $request->query->get('page');\n// $request->query->get('commented')\n" +
				"$m = (int)$a-$request->query->get('offset');\n$l = (int) strlen($request->request->get('comment'));\n" +
				"$p = $request->request->getDigits('phone');",
			"web/modules/contrib/shop/shop.module": "<?php\nfunction shop_x() { $c = \\Drupal::request()->query->getAlnum('coupon'); }",
		})
		sameList(t, "drupal", inputs(s), "comment request unknown", "coupon query key", "email request unknown", "offset query unknown", "page query int",
			"phone body unknown", "product query int", "q query unknown")
	})
	t.Run("mediawiki", func(t *testing.T) {
		s := read(t, map[string]string{
			"includes/Defines.php":           "<?php define( 'MW_VERSION', '1.43.0' );",
			"api.php":                        acting,
			"extensions/Quiz/extension.json": `{"name":"Quiz","version":"1.0"}`,
			"extensions/Quiz/includes/Special.php": "<?php\n$id = $this->getRequest()->getInt( 'quiz' );\n" +
				"$t = $req->getText( 'answer' );\n$ok = $req->getBool( 'confirm' );\n$v = $req->getVal( 'sort' );",
		})
		sameList(t, "mediawiki", inputs(s), "answer request text", "confirm request bool", "quiz request int", "sort request unknown")
	})
	t.Run("phpbb", func(t *testing.T) {
		s := read(t, map[string]string{
			"includes/constants.php": "<?php @define('PHPBB_VERSION', '3.3.15');",
			"viewtopic.php":          acting,
			"ext/phpbb/boardrules/controller/main.php": "<?php\n$id = $this->request->variable('rule_id', 0);\n$lat = $request->variable('lat', 0.0);\n" +
				"$t = $this->request->variable('title', '', true);\n$ok = $request->variable('agree', false);",
			"ext/phpbb/viglink/event/listener.php": "<?php $x = $this->request->variable('core', 0);",
		})
		sameList(t, "phpbb", inputs(s), "agree request bool", "lat request unknown", "rule_id request int", "title request text")
	})
	t.Run("typo3", func(t *testing.T) {
		s := read(t, map[string]string{
			"typo3/sysext/core/Classes/Information/Typo3Version.php": "<?php final class Typo3Version { protected const VERSION = '13.4.1'; }",
			"typo3conf/ext/news/Classes/Controller/NewsController.php": "<?php\n$n = $this->request->getArgument('news');\n" +
				"$q = $request->getQueryParams()['tx_search'];\n$p = (int) GeneralUtility::_GP('page');",
		})
		sameList(t, "typo3", inputs(s), "news request unknown", "page request int", "tx_search request unknown")
	})
	t.Run("opencart and prestashop and magento", func(t *testing.T) {
		oc := read(t, map[string]string{
			"index.php": "<?php define('VERSION', '4.0.2.3');", "system/startup.php": "<?php",
			"catalog/controller/common/home.php":                    "<?php class Home {}",
			"extension/acme/catalog/controller/module/wishlist.php": "<?php\n$id = (int)$this->request->get['product_id'];\n$e = $this->request->post['email'];",
		})
		sameList(t, "opencart", inputs(oc), "email body unknown", "product_id query int")
		ps := read(t, map[string]string{
			"classes/ObjectModel.php": acting, "config/defines.inc.php": "<?php",
			"src/Core/Version.php":        "<?php class Version { public const VERSION = '8.2.0'; }",
			"modules/loyalty/loyalty.php": "<?php if (!defined('_PS_VERSION_')) { exit; }\n$id = (int) Tools::getValue('id_order');\n$c = Tools::getValue('code');",
		})
		sameList(t, "prestashop", inputs(ps), "code request unknown", "id_order request int")
		mg := read(t, map[string]string{
			"app/bootstrap.php": "<?php", "app/etc/NonComposerComponentRegistration.php": "<?php",
			"app/code/Magento/Catalog/Controller/Product/View.php": "<?php $x = $this->getRequest()->getParam('core');",
			"app/code/Acme/Quote/Controller/Request/Send.php": "<?php\n$id = (int) $this->getRequest()->getParam('quote_id');\n" +
				"$e = $this->getRequest()->getPost('email');",
		})
		sameList(t, "magento", inputs(mg), "email body unknown", "quote_id request int")
	})
	t.Run("not on another application", func(t *testing.T) {
		s := read(t, map[string]string{"index.php": "<?php $x = $request->query->getInt('x'); echo 1;"})
		sameList(t, "custom site", inputs(s)) // Symfony's reader is read where Symfony is
	})
}

// A value the reader made a number leaves nothing harmful: no risky call;
// one kept as given does.
func TestRequestReadersTaint(t *testing.T) {
	s := read(t, map[string]string{
		"web/core/lib/Drupal.php": "<?php class Drupal { const VERSION = '11.1.0'; }",
		"web/modules/contrib/shop/src/Repo.php": "<?php\n$id = $request->query->getInt('id');\n$db->query(\"select * from x where id = $id\");\n" +
			"$q = $request->query->get('q');\n$db->query(\"select * from x where name = '$q'\");",
	})
	var got []string
	for _, r := range s.Risks {
		got = append(got, r.Kind+" "+r.Input)
	}
	sameList(t, "risks", got, "sql q")
}
