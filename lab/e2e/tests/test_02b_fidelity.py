"""The same pages seen directly on WordPress, Joomla, Drupal and a custom
application and through Equalizer must look the same: pixels, visible text
and resources that load."""
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import fidelity  # noqa: E402

# What legitimately changes between two visits: clocks, timers and the
# number of new drafts.
VOLATILE = re.compile(r"^[+-](.*\b\d{1,2}:\d{2}\b.*|.*\b\d+ (seconds?|minutes?)\b.*|\d+)$")


def test_pages_look_the_same(playwright):
    for pct, textdiff, only_gateway, path, shows_address in fidelity.main(playwright):
        assert all(VOLATILE.match(line) for line in textdiff), (path, textdiff[:5])
        # A clock, a timer or the site's address written out (https has one
        # more letter) redraws a few pixels; nothing else may.
        assert pct < (1.0 if textdiff or shows_address else 0.1), f"{path}: {pct:.2f}% of pixels differ"
        assert not only_gateway, (path, only_gateway[:5])
