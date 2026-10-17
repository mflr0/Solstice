# Security policy

## Supported versions

Only the latest release is supported. Before 1.0, the supported code is on the
`feat/v1.0` branch.

## Reporting a vulnerability

Report it privately. In the GitHub repository, open the Security tab and use
the "Report a vulnerability" button. Never write it in a public issue.

Include three things:

- The version: the output of `solstice version`.
- The steps to reproduce the problem.
- The impact: what an attacker could gain.

## What happens next

We acknowledge the report, reproduce it, prepare a fix and publish a release
with the fix. If you want to be named in the release notes, say so in your
report; if you do not, the release does not name you.

## Release integrity

`scripts/release.sh` writes `SHA256SUMS`, the SHA-256 checksum of every
archive of the release, and signs that file with Ed25519. The signature comes
from `SHA256SUMS.sig`. The private key stays outside the repository, named by
`SOLSTICE_RELEASE_KEY`; the public key is written in the update helper
(`deploy/linux/update.sh`).

The update helper only downloads from the GitHub releases of the project. It
verifies `SHA256SUMS.sig` against its public key before it reads any
checksum. A missing or wrong signature, or a helper that still holds the
placeholder key, refuses the update. It then checks the archive against its
line in `SHA256SUMS`, and the binary inside against the requested version.
Only a version newer than the installed one is accepted, never an older one.
