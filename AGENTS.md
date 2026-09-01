# Project working agreement

This file supplements the shared global agent guidance with rules specific to this repository.

## Scope and safety

- This repository contains a Home Assistant add-on and its Go voice server. Keep add-on and server changes minimal and compatible with existing Xiaozhi/OPUS devices.
- Before editing, run `git status --short --branch`. Preserve unrelated changes and untracked files. In particular, do not include `.DS_Store` or local `docs/context/` handoff files unless the user explicitly asks.
- Do not expose API keys, Home Assistant tokens, device identifiers, or local network addresses in commits, documentation, issues, or releases.
- Do not commit, push, create releases, or comment on/close GitHub issues unless the user has authorized that external action.

## Verification

- For changes under `stackchan-server/server/internal/service/ai`, run the affected package tests. Add a regression test for a fixed protocol or configuration bug when practical.
- Before a user-facing release, run the relevant Go tests, `go build ./...`, `bash -n stackchan-server/run.sh`, YAML parsing for add-on configuration, and `git diff --check`.
- Report unavailable checks precisely. Existing unrelated failures must be named and kept out of scope unless the user asks to fix them.

## Documentation and releases

- Keep `README.md`, `README.zh.md`, and `CHANGELOG.md` aligned with user-visible configuration changes.
- Record substantial architecture decisions in `docs/`. Keep local handoff records under `docs/context/` when continuity is needed.
- Every user-facing add-on release must update `stackchan-server/config.yaml` and `CHANGELOG.md`, then follow this order: merge/push final `main`, create a matching `v<version>` tag on that commit, publish a non-draft GitHub Release with matching notes, and verify the remote manifest, tag target, Release state, and notes.

## Standalone mode

- Home Assistant support is optional only after the no-HA path has equivalent device-session, provider, configuration, and security coverage.
- Prefer the dual-runtime plan in `docs/standalone-mode-plan.md`; do not replace this server wholesale with an upstream project without a documented compatibility and licensing review.
