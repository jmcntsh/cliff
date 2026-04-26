# Seeding `cliff-registry` from GitHub stars

This repo now includes `scripts/seed_registry_from_github.py` to
build a review queue of candidate CLI/TUI repos from GitHub.

It is designed for high-volume discovery, not blind import:

- Pull top repos created in the last year (by stars).
- Deduplicate by repo URL.
- Score confidence with simple CLI/TUI heuristics.
- Output machine-readable candidates + human-review CSV.
- Optionally emit ready-to-lint Go manifests for medium/high confidence repos.
- Apply deny/allow rules before scoring and manifest emission.

## Prereqs

- `gh` CLI installed and authenticated (`gh auth status`).
- Python 3.11+ (uses stdlib `tomllib`).
- Optional: local checkout of `jmcntsh/cliff-registry` for dedupe.

## 1) Generate candidate queue (1000 target)

```sh
python3 scripts/seed_registry_from_github.py \
  --since "$(date -v-1y +%F)" \
  --min-stars 75 \
  --limit 1000 \
  --rules-file scripts/seeding-rules.toml \
  --out-dir /tmp/cliff-seed-2026-04
```

Outputs:

- `/tmp/cliff-seed-2026-04/candidates.json`
- `/tmp/cliff-seed-2026-04/review.csv`

`review.csv` is the primary triage file (sort/filter by confidence,
score, language, stars).

## 2) Emit draft import batch for Go repos (optional)

If you keep a `cliff-registry` checkout next to this repo:

```sh
python3 scripts/seed_registry_from_github.py \
  --since "$(date -v-1y +%F)" \
  --min-stars 75 \
  --limit 1000 \
  --rules-file scripts/seeding-rules.toml \
  --out-dir /tmp/cliff-seed-2026-04 \
  --registry-dir ../cliff-registry \
  --emit-go-manifests \
  --max-go-manifests 40
```

Additional output:

- `/tmp/cliff-seed-2026-04/manifests-go/*.toml`
- `/tmp/cliff-seed-2026-04/manifest-batch.csv` (metadata for emitted manifests)

These manifests use:

- `homepage = "https://github.com/<owner>/<repo>"`
- `readme = "https://raw.githubusercontent.com/<owner>/<repo>/<default-branch>/README.md"`
- `[install] type = "go"` with `package = "github.com/<owner>/<repo>@latest"`

## 3) Import to `cliff-registry`

```sh
cp /tmp/cliff-seed-2026-04/manifests-go/*.toml ../cliff-registry/apps/
cd ../cliff-registry
go run ./cmd/lint ./apps
go run ./cmd/build ./apps /tmp/index.json
```

Then open a PR in `cliff-registry` with a batch size you can review
quickly (for example 25-75 apps per PR).

## Rules file

Default rules live in `scripts/seeding-rules.toml`.

- `[deny].terms` drops repos when name/description/full name contains a term.
- `[deny].owners` drops everything from specific owners.
- `[deny].name_patterns` drops repos matching regex against `owner/name`.
- `[allow]` values do not force inclusion; they are appended to `why` so
  triage can prioritize likely hits.

## Notes and limitations

- GitHub does not expose a global "most cloned" public leaderboard.
- "Past year" here means repos created after `--since`.
- Install inference for non-Go ecosystems is only a suggestion in
  `review.csv`; no manifests are auto-emitted for them.
- Heuristics intentionally prefer false negatives over false positives.
