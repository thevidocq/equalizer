"""Magento and Adobe Commerce (2.x)."""
import re

from .base import Profile

PROFILE = Profile(
    name="magento", title="Magento / Adobe Commerce",
    generator=re.compile(r"magento\s*([\d.]+)?", re.I),
    paths=re.compile(r"/static/(version\d+/)?(frontend|adminhtml)/|/pub/static/|mage/cookies|Magento_"),
    components=re.compile(r"/static/(?:version\d+/)?frontend/[^/]+/[^/]+/[^/]+/()([A-Z][A-Za-z0-9]+_[A-Za-z0-9]+)/"),
    login="/customer/account/login/",
    # The administration's address is chosen at install time (often secret):
    # the usual ones are closed, the real one stays with the site area.
    admin=(r"^/(index\.php/)?admin(_[a-z0-9]+)?(/|$)", r"^/(rest|soap)(/|$)", r"^/setup(/|$)", r"^/downloader(/|$)"),
    never=(r"^/checkout/cart/(add|delete|updatePost)", r"^/wishlist/index/(add|remove)", r"^/customer/account/logout",
           r"^/catalog/product_compare/(add|remove|clear)", r"^/sendfriend/", r"^/newsletter/subscriber/"),
    note="Administration closed: its address differs per installation. Cart, wishlist and comparison links "
         "are never opened.",
)
