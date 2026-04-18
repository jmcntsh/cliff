# cliff registry

Manifests for apps listed in [cliff](https://cliff.sh).

This directory will eventually move to its own repo (`jmcntsh/cliff-registry`
or similar). Until then it lives here so the scaffold can evolve alongside
the client.

## Layout

```
registry/
  apps/                 — one TOML manifest per app
  cmd/lint/             — manifest validator (also used by CI)
  cmd/build/            — compiles apps/*.toml into index.json
  schema/manifest.md    — schema spec (mirrors notes/manifest.md)
  index.json            — generated; do not edit by hand
```

## Workflow

1. Add or edit a manifest under `apps/<name>.toml`.
2. Run `go run ./registry/cmd/lint ./registry/apps` to validate.
3. Run `go run ./registry/cmd/build ./registry/apps ./registry/index.json`
   to regenerate the compiled index.
4. Commit both the manifest and the regenerated `index.json`.

When this directory moves to its own repo, GitHub Actions will run
lint on PRs and rebuild `index.json` on merge to main, then publish via
GitHub Pages at `https://registry.cliff.sh/index.json`.

## Schema

See [`../notes/manifest.md`](../notes/manifest.md) for the canonical spec.
