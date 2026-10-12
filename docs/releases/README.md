# Release notes

This directory holds the user-facing release notes for `appsumo-cli`, one file
per version, following the style established by [`v0.2.0.md`](v0.2.0.md).

## Source of truth

[`CHANGELOG.md`](../../CHANGELOG.md) is the single source of truth (SSOT) for
what shipped in each version and on what date. The files here are the
CHANGELOG-aligned, user-facing digest, and they are what the GitHub release body
is published from:

```bash
gh release create v0.5.5 --title "..." --notes-file docs/releases/v0.5.5.md --verify-tag
```

Because these files are tracked, they fall inside the repository's existing
gates: `TestDocumentedCommandsParse` checks every `appsumo ...` string in them
against the real parser, and `TestTrackedMarkdownHasNoCredentialShapedLiterals`
scans them for pasted secrets.

## Index

| Version | Date | Release note | Git tag |
| --- | --- | --- | --- |
| 0.1.0 | 2026-06-16 | [`v0.1.0.md`](v0.1.0.md) | never tagged (historical) |
| 0.2.0 | 2026-08-14 | [`v0.2.0.md`](v0.2.0.md) | [`v0.2.0`](https://github.com/vecyang1/appsumo-cli/releases/tag/v0.2.0) |
| 0.3.0 | 2026-09-10 | [`v0.3.0.md`](v0.3.0.md) | not yet tagged |
| 0.4.0 | 2026-10-07 | [`v0.4.0.md`](v0.4.0.md) | not yet tagged |
| 0.4.1 | 2026-10-07 | [`v0.4.1.md`](v0.4.1.md) | not yet tagged |
| 0.5.0 | 2026-10-07 | [`v0.5.0.md`](v0.5.0.md) | not yet tagged |
| 0.5.1 | 2026-10-07 | [`v0.5.1.md`](v0.5.1.md) | not yet tagged |
| 0.5.2 | 2026-10-08 | [`v0.5.2.md`](v0.5.2.md) | not yet tagged |
| 0.5.3 | 2026-10-08 | [`v0.5.3.md`](v0.5.3.md) | not yet tagged |
| 0.5.4 | 2026-10-08 | [`v0.5.4.md`](v0.5.4.md) | not yet tagged |
| 0.5.5 | 2026-10-08 | [`v0.5.5.md`](v0.5.5.md) | not yet tagged |

## Version and tag status

`CHANGELOG.md` currently records versions through **0.5.5**, while the newest
Git tag is **v0.2.0**. The gap is expected in the sense that changelog entries
land with the commit that ships them, but it means the intervening versions have
release notes without a corresponding tag or GitHub release.

Cutting tags and publishing releases is intentionally **out of scope** here — it
requires separate authorization. When a tag is cut, follow the workflow in
[`AGENTS.md`](../../AGENTS.md#releasing): tag the commit CI passed on, publish
the release body from the matching file in this directory, and confirm the
installed binary reports the tag.

Per `AGENTS.md`, there is no `v0.1.0` tag: that release exists only as a
`CHANGELOG.md` section, and `v0.2.0` is the first tagged release. Its note is
kept here for a complete, gap-free version history.

---

This is an unofficial community tool, not affiliated with or endorsed by AppSumo.
