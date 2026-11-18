# xhtml

Copy of the `golang.org/x/net/html` HTML5 parser (with `atom`), version
**v0.40.0**, BSD licence (see `LICENSE`). Tests, test data and the `atom`
generator are excluded; the only change is the `atom` import path.

It is kept in the repository because the development environments in use
cannot always reach the Go module proxy. To update it: copy the files from
the new version, repeat the import change, run the gateway tests.
