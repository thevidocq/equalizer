"""The same pages seen directly on WordPress and through Equalizer must
look the same: pixels, visible text and resources that load."""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import fidelity  # noqa: E402

# What legitimately changes between two visits: the clock and new drafts.
VOLATILE = re.compile(r"^[+-](Universal time is .*|\d+)$")


def test_pages_look_the_same(playwright):
    for pct, textdiff, only_gateway, path in fidelity.main(playwright):
        assert pct < 0.1, f"{path}: {pct:.2f}% of pixels differ"
        assert all(VOLATILE.match(line) for line in textdiff), (path, textdiff[:5])
        assert not only_gateway, (path, only_gateway[:5])
