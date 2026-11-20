"""What a page tells: links, forms, scripts, assets and generator."""

import re
import urllib.parse
from dataclasses import dataclass, field
from html.parser import HTMLParser


@dataclass
class Input:
    """A form field with what its HTML says about its values."""
    name: str
    type: str = "text"
    required: bool = False
    maxlength: int = 0
    options: list = field(default_factory=list)  # select options, radio and checkbox values
    pattern: str = ""
    min: str = ""
    max: str = ""
    step: str = ""
    value: str = ""  # a hidden field's or a button's own value


@dataclass
class Form:
    action: str  # absolute URL
    method: str
    inputs: list = field(default_factory=list)
    page: str = ""  # the page that shows it (absolute URL)

    def visible(self):
        return [i for i in self.inputs if i.type not in ("hidden", "submit", "button", "image", "reset")]

    def hidden(self):
        return [i for i in self.inputs if i.type == "hidden"]


@dataclass
class Page:
    links: list = field(default_factory=list)
    assets: list = field(default_factory=list)
    scripts: list = field(default_factory=list)
    forms: list = field(default_factory=list)
    generator: str = ""
    title: str = ""


# URLs in scripts: quoted strings that look like paths of the site.
SCRIPT_URL = re.compile(r"""["'](/[A-Za-z0-9_\-./]{1,200}(?:\?[A-Za-z0-9_\-=&.%]{0,200})?)["']""")


class _Parser(HTMLParser):
    def __init__(self, base: str):
        super().__init__(convert_charrefs=True)
        self.base, self.page, self.form, self.select, self.in_title, self.in_script = base, Page(), None, None, False, False

    def _abs(self, href: str) -> str:
        return urllib.parse.urljoin(self.base, href.strip())

    def handle_starttag(self, tag, attrs):
        a = {k: (v or "") for k, v in attrs}
        if tag == "base" and a.get("href"):
            self.base = self._abs(a["href"])
        elif tag in ("a", "area") and a.get("href"):
            self.page.links.append(self._abs(a["href"]))
        elif tag == "link" and a.get("href"):
            rel = a.get("rel", "").lower()
            (self.page.assets if "stylesheet" in rel or "icon" in rel or "preload" in rel else self.page.links).append(self._abs(a["href"]))
        elif tag in ("img", "source", "video", "audio") and a.get("src"):
            self.page.assets.append(self._abs(a["src"]))
        elif tag == "script":
            self.in_script = True
            if a.get("src"):
                self.page.scripts.append(self._abs(a["src"]))
        elif tag == "iframe" and a.get("src"):
            self.page.links.append(self._abs(a["src"]))
        elif tag == "meta" and a.get("name", "").lower() == "generator":
            self.page.generator = a.get("content", "")[:100]
        elif tag == "title":
            self.in_title = True
        elif tag == "form":
            self.form = Form(self._abs(a.get("action") or self.base), (a.get("method") or "get").upper())
            self.page.forms.append(self.form)
        elif tag in ("input", "textarea", "select") and self.form is not None and a.get("name"):
            t = "textarea" if tag == "textarea" else "select" if tag == "select" else (a.get("type") or "text").lower()
            if t in ("radio", "checkbox"):
                # one field, the values of its boxes
                value = a.get("value") or "on"
                same = next((i for i in self.form.inputs if i.name == a["name"] and i.type == t), None)
                if same is not None:
                    if value not in same.options:
                        same.options.append(value)
                    same.required = same.required or "required" in a
                    return
                self.form.inputs.append(Input(a["name"], t, "required" in a, options=[value]))
                return
            ml = a.get("maxlength", "")
            inp = Input(a["name"], t, "required" in a, int(ml) if ml.isdigit() else 0,
                        pattern=a.get("pattern", "")[:300], min=a.get("min", ""), max=a.get("max", ""), step=a.get("step", ""),
                        value=a.get("value", "") if t in ("submit", "image", "hidden") else "")
            self.form.inputs.append(inp)
            if tag == "select":
                self.select = inp
        elif tag == "option" and self.select is not None:
            self.select.options.append(a.get("value", ""))

    def handle_endtag(self, tag):
        if tag == "form":
            self.form = None
        elif tag == "select":
            self.select = None
        elif tag == "title":
            self.in_title = False
        elif tag == "script":
            self.in_script = False

    def handle_data(self, data):
        if self.in_title and len(self.page.title) < 120:
            self.page.title += data.strip()
        elif self.in_script:
            self.page.links.extend(self._abs(m) for m in SCRIPT_URL.findall(data))


def parse(html: str, base: str) -> Page:
    p = _Parser(base)
    try:
        p.feed(html)
        p.close()
    except Exception:  # broken markup: keep what was read
        pass
    for f in p.page.forms:
        f.page = base
    return p.page


def script_urls(js: str, base: str) -> list:
    return [urllib.parse.urljoin(base, m) for m in SCRIPT_URL.findall(js)]


SITEMAP_LOC = re.compile(r"<loc>\s*([^<\s]+)\s*</loc>", re.I)


def sitemap_urls(xml: str) -> list:
    return SITEMAP_LOC.findall(xml)[:2000]
