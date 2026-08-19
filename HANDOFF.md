# Handoff Log

Append-only. Every agent writes here on task completion.

---

## 2026-08-19 Dev → docs and release artifacts (pgoctl public-readiness)

Status: done
Output:
- `CHANGELOG.md` committed to main (Keep a Changelog format, `[0.1.0-alpha] - 2026-08-19` section covering all subcommands, Parca adapter, config system, and CI/bench workflows; no internal D-numbers in visible text)
- `README.md` badge row added (CI, Go Report Card, pkg.go.dev, License, Latest Release — in spec order, with pre-launch activation note)
- Annotated tag `v0.1.0-alpha` on main HEAD `501acef` — <https://github.com/better-go-labs/pgoctl/releases/tag/v0.1.0-alpha>

Notes: All three artifacts share the same version string (`v0.1.0-alpha`). Release is marked pre-release. PR chain #12→#22 not touched. Badges that depend on public visibility will be inactive until Gyanesh flips repo visibility.
