# Screenshots

The manuals' screenshots (`docs/screenshots`) come from the panel of the
commit, filled with a demo organisation (Acme, domains under `example`)
through the panel's own API, never by hand:

- `stack.sh`: panel-api on an empty database with throwaway keys, the
  platform administrator, then `seed.py`;
- `seed.py`: people, connector, two sites, discoveries with the reports
  and maps of real studies of the lab (`fixtures/`, from
  `lab/e2e/efficacy`), a verification with a correction, a draft in
  review, a certificate, a source reader with a read;
- `logs.py`: the gateway's security events of a real study, into Loki;
- `shoot.py`: the pages, with Playwright, at 1360 px.

CI runs it (`.github/workflows/screenshots.yml`): on a push to a
`docs/screenshots-*` branch it checks that every page loads; run by hand
on such a branch (`gh workflow run screenshots.yml --ref <branch>`), it
also commits the images and the in-app help to it. Then push any commit
of your own to the branch, so that its pull request has its checks.
