"""The lab sites the efficacy study measures: one per kind of site the
panel offers (its map templates), the custom app for the generic one. Each
is served again here, on its own port, behind record.php, so that what
reaches it is logged. A site the lab has not built (wordpress.sh, cms.sh,
webapps.sh) is listed as not studied."""
import os
import pathlib

LAB = pathlib.Path(__file__).resolve().parent.parent
RUN = LAB / ".run"
OUT = pathlib.Path(os.environ.get("STUDY_OUT", RUN / "efficacy"))
ROOT = LAB.parent.parent
PASSWORD = "Lab-editor-pass-1"

SITES = {
    "wordpress": {"host": "www.acme.test", "legacy": 18190, "gateway": 18290, "root": RUN / "wordpress",
                  "router": LAB / "router.php", "account": "editor"},
    "joomla": {"host": "news.acme.test", "legacy": 18191, "gateway": 18291, "root": RUN / "joomla",
               "router": LAB / "router.php", "account": "editor"},
    "drupal": {"host": "docs.acme.test", "legacy": 18192, "gateway": 18292, "root": RUN / "drupal",
               "router": RUN / "drupal" / ".ht.router.php", "cwd": RUN / "drupal", "account": "editor"},
    "generic": {"host": "intranet.acme.test", "legacy": 18193, "gateway": 18293, "root": RUN / "intranet",
                "env": {"INTRANET_DB": str(RUN / "intranet-db" / "intranet.sqlite")}, "account": "editor"},
    "opencart": {"host": "cart.acme.test", "legacy": 18194, "gateway": 18294, "root": RUN / "cart",
                 "account": "editor@acme.test"},
    "prestashop": {"host": "shop.acme.test", "legacy": 18195, "gateway": 18295, "root": RUN / "shop",
                   "account": "editor@acme.test"},
    "phpbb": {"host": "forum.acme.test", "legacy": 18196, "gateway": 18296, "root": RUN / "forum", "account": "editor"},
    "mediawiki": {"host": "wiki.acme.test", "legacy": 18197, "gateway": 18297, "root": RUN / "wiki", "account": "editor"},
    "typo3": {"host": "site.acme.test", "legacy": 18198, "gateway": 18298, "root": RUN / "site",
              "router": LAB / "router.php", "account": None},
    "magento": {"host": "store.acme.test", "legacy": 18199, "gateway": 18299, "root": RUN / "store" / "pub",
                "router": RUN / "store" / "phpserver" / "router.php", "cwd": RUN / "store" / "pub",
                "account": "editor@acme.test"},
}
MARKERS = {"wordpress": "wp-config.php", "joomla": "configuration.php", "drupal": "sites/default/settings.php",
           "generic": "index.php", "opencart": "config.php", "prestashop": "app/config/parameters.php",
           "phpbb": "config.php", "mediawiki": "LocalSettings.php", "typo3": "typo3conf/system/settings.php",
           "magento": "index.php"}


def available(name: str) -> bool:
    """Whether the lab has built the site (wordpress.sh, cms.sh)."""
    return (SITES[name]["root"] / MARKERS[name]).exists()
