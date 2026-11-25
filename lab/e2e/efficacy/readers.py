"""Reader parity: the source reader on each kind of site with one real,
widely used extension added, so that every application's extension code is
read as WordPress plugins are (docs/efficacy.md, "Source code").

    python3 lab/e2e/efficacy/readers.py

Each lab site is copied with hard links (nothing in it changes), the
extension (a pinned GitHub tag) goes where its application installs it, and
source-summary reads the copy. Prints a table and writes
.run/efficacy/readers.md; exits 1 when an extension gives no entry point and
no input."""
import collections
import json
import shutil
import subprocess
import sys

from sites import OUT, RUN, SITES, available
import stack

SRC = RUN / "src" / "extensions"
# site: (repository, tag, folders of the repository → where they go in the site)
EXTENSIONS = {
    "wordpress": ("awesomemotive/easy-digital-downloads", "3.7.3", {".": "wp-content/plugins/easy-digital-downloads"}),
    "joomla": ("joomla-extensions/weblinks", "5.2.1", {"src/components/com_weblinks": "components/com_weblinks",
               "src/administrator/components/com_weblinks": "administrator/components/com_weblinks",
               "src/api/components/com_weblinks": "api/components/com_weblinks"}),
    "drupal": ("drupal-graphql/graphql", "8.x-4.14", {".": "modules/contrib/graphql"}),
    "phpbb": ("phpbb-extensions/boardrules", "3.1.0", {".": "ext/phpbb/boardrules"}),
    "mediawiki": ("wikimedia/mediawiki-extensions-PageForms", "REL1_43", {".": "extensions/PageForms"}),
    "typo3": ("georgringer/news", "14.1.2", {".": "typo3conf/ext/news"}),
}


def fetch(repo: str, tag: str):
    d = SRC / repo.replace("/", "_") / tag
    if not d.exists():
        d.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(["git", "-c", "advice.detachedHead=false", "clone", "-q", "--depth", "1", "--branch", tag,
                        f"https://github.com/{repo}", str(d)], check=True)
    return d


def read(name: str) -> dict:
    repo, tag, parts = EXTENSIONS[name]
    ext = fetch(repo, tag)
    copy = OUT / "readers" / name
    shutil.rmtree(copy, ignore_errors=True)
    copy.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(["cp", "-al", str(SITES[name]["root"]), str(copy)], check=True)
    for src, dst in parts.items():
        # the copy shares the site's files: a folder already there is
        # replaced, never written into (that would change the site itself)
        if (copy / dst).is_symlink() or (copy / dst).is_file():
            (copy / dst).unlink()
        shutil.rmtree(copy / dst, ignore_errors=True)
        shutil.copytree(ext / src, copy / dst, ignore=shutil.ignore_patterns(".git"))
    out = subprocess.run([str(OUT / "source-summary"), str(copy)], capture_output=True, text=True, check=True).stdout
    shutil.rmtree(copy)
    s = json.loads(out)
    owner = repo.split("/")[1]
    return {"extension": f"{repo} {tag}", "stack": s["stack"], "files": s["files"]["read"],
            "routes": len(s["routes"] or []), "inputs": len(s["inputs"] or []),
            "types": dict(collections.Counter(i["type"] for i in s["inputs"] or [])),
            "risks": len(s["risks"] or []), "owner": owner}


def main():
    stack.build()
    rows, failed = {}, []
    for name in EXTENSIONS:
        if not available(name):
            continue
        rows[name] = r = read(name)
        if not r["routes"] and not r["inputs"]:
            failed.append(name)
    lines = ["# Reader parity", "", "| Site | Extension | Stack | Files read | Entry points | Inputs | Types | Risky calls |",
             "| --- | --- | --- | --- | --- | --- | --- | --- |"]
    for name, r in rows.items():
        types = ", ".join(f"{k} {n}" for k, n in sorted(r["types"].items()))
        lines.append(f"| {name} | {r['extension']} | {r['stack']} | {r['files']} | {r['routes']} | {r['inputs']} | {types} | {r['risks']} |")
    if failed:
        lines += ["", "FAILED: no entry point and no input read in " + ", ".join(failed)]
    (OUT / "readers.md").write_text("\n".join(lines) + "\n")
    print("\n".join(lines))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
