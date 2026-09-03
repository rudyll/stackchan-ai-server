# Licensing and voluntary sponsorship

## Decision — 2026-09-03

The maintainer approved **AGPL-3.0-only** for the current combined project and
voluntary sponsorship as the funding model. This is not a non-commercial
license. Commercial users who comply do not owe a donation or profit share.
There is no automatic proprietary/dual-license offer; any future exception needs
adequate rights and a separately reviewed agreement. See [CONTRIBUTING](../CONTRIBUTING.md).

The root and add-on LICENSE files contain the identical GNU AGPLv3 text.
[NOTICE](../stackchan-server/NOTICE.md) defines the scope, copyright attribution,
and retained MIT permissions. Do not overwrite historical MIT release artifacts,
rewrite their notes as AGPL, or remove original third-party notices.

## Engineering license inventory

Checked the tracked history and the root license/notice files of all 37 external
Go modules selected by `go list -mod=readonly -deps ./...` at the transition.
This is a scoped engineering check, not a legal opinion or exhaustive audit of
all assets, patents, jurisdictions and per-file notices. Dependencies were not
changed for this transition. Versions remain pinned in go.mod/go.sum.

- M5Stack-derived source: MIT; original notices retained. The one non-maintainer
  commit in the checked history changes go.mod under the then-current MIT terms;
  its attribution and prior permissions remain intact.
- MIT: GoFrame, golang-jwt, Go Opus bindings, TOML, xxhash, mxj, clipperhouse
  displaywidth/uax29, fatih/color, goccy/go-json, go-colorable, go-isatty,
  go-runewidth, olekukonko cat/errors/ll/tablewriter.
- BSD-family: Go standard library, edwards25519, fsnotify, google/uuid,
  gorilla/websocket, html-strip-tags-go, properties and golang.org/x/net/sys/text.
  gods/v2 carries BSD and an additional ISC notice; preserve both.
- Apache-2.0: go-logr/logr and stdr, OpenTelemetry auto/sdk and otel modules.
  The latter also carry a Go BSD notice. yaml.v3 carries MIT/Apache notices.
- `github.com/go-sql-driver/mysql v1.9.3`: **MPL-2.0**, not MIT. Its actual source
  headers contain no attached "Incompatible With Secondary Licenses" notice
  (the standard LICENSE includes Exhibit B as a template). Preserve MPL notices
  and source availability; use section 3.3 when distributing the combined AGPL
  work. Do not relabel the dependency as exclusively AGPL.
- Native OPUS 1.6.1 used by the macOS builder: BSD-style COPYING plus the listed
  royalty-free patent notices. Container native-library versions come from
  Alpine; preserve and review their own notices when preparing a distribution.

Primary references: [GNU compatibility](https://www.gnu.org/licenses/license-compatibility.en.html),
[Mozilla MPL terms, sections 1.12/3.3](https://www.mozilla.org/en-US/MPL/2.0/),
[AGPL section 13](https://www.gnu.org/licenses/agpl.en.html#section13).

## Release/source-availability checklist

1. Review new dependencies and copied code, retain original notices, and include
   the project license and third-party licenses in distributed packages.
2. Identify the exact source revision for each distributed binary/container.
   Provide the complete corresponding source, including modifications and
   required build/install scripts and dependency source as applicable. For
   packaged releases, prepare and verify a matching source archive; a repository
   homepage or moving main branch alone is insufficient.
3. Keep a prominent source link in network-facing interfaces. Operators of
   modified services must update that offer to their exact source and make it
   available to the users covered by section 13, not merely link upstream.
4. Source offers must not require sponsorship or transfer of copyright. AGPL
   compliance does not force a downstream PR to this project.
5. Before publishing a future AGPL DMG/container release, verify the source
   download, build correspondence, license bundle and user-visible source offer.
   The transition does not claim that old MIT artifacts were rebuilt as AGPL.

## Funding workflow

[SPONSORING](../SPONSORING.md) is the bilingual public policy. The maintainer
confirmed `https://paypal.me/unitekno` on 2026-09-03; no QR code is published
for this account. The link is
configured as a custom URL in `.github/FUNDING.yml` and displayed near the top
of both READMEs and the add-on README. This uses an external PayPal
destination, not GitHub Sponsors payment enrollment.

The repository's **Settings → General → Features → Sponsorships** option must
also be enabled. Adding `.github/FUNDING.yml` alone does not turn on a disabled
Sponsor button. Verify both the enabled setting and the public button/link.

Only add payment destinations supplied and approved by the maintainer. Verify
the destination and rendered Sponsor button after any update. Never
publish placeholders or guessed PayPal usernames. Cryptocurrency requires
explicit currency/network/address approval; no crypto address is published.

This project-specific license decision does not mandate AGPL for every future
project; voluntary sponsorship is the shared default, licenses remain scoped.
