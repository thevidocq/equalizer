# pkcs12 (vendored)

PKCS#12 (`.pfx`, `.p12`) decoding, used to read the certificates customers
upload in the panel (IIS exports them this way).

- Upstream: `software.sslmate.com/src/go-pkcs12`
  (https://github.com/SSLMate/go-pkcs12), commit `0bb5b30`.
- Licence: BSD-3-Clause, see `LICENSE`.
- Why vendored: the module needs `golang.org/x/crypto`, which the
  development environment cannot download. The only change is
  `stdpbkdf2.go`: PBKDF2 comes from the standard library (`crypto/pbkdf2`,
  Go 1.24) instead of `golang.org/x/crypto/pbkdf2`; import paths point
  here. Tests and test data are not copied.

To update: copy the upstream `*.go` files (not the tests) and
`internal/rc2`, then apply the same two changes.
