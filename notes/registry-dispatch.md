# Registry → cliff snapshot refresh

The client ships with `internal/catalog/data/index.json` as a
build-time snapshot of `registry.cliff.sh/index.json`. That file is
used on first launch (before the live fetch returns) and as the
offline fallback. It drifts from the live registry whenever new apps
land, and a stale snapshot is a bad first impression.

The `.github/workflows/refresh-snapshot.yml` workflow in the `cliff`
repo opens a PR to bump the snapshot whenever the live registry
changes. It has three triggers: a weekly schedule, a manual button,
and a `repository_dispatch` event fired from this registry repo.

## Registry-side snippet

Add this step to the `cliff-registry` repo's "build-index" workflow —
the one that publishes `index.json` to GitHub Pages — immediately
after the publish step:

```yaml
  notify-cliff-client:
    needs: [publish] # or whatever the publish job is called
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Kick cliff snapshot refresh
        env:
          GH_TOKEN: ${{ secrets.CLIFF_CLIENT_DISPATCH_TOKEN }}
        run: |
          gh api \
            -X POST \
            -H "Accept: application/vnd.github+json" \
            /repos/jmcntsh/cliff/dispatches \
            -f event_type=registry-published
```

`CLIFF_CLIENT_DISPATCH_TOKEN` needs to be a fine-grained PAT on the
cliff client repo with **Actions: Read and write**. That's the one
manual step that has to exist; the rest of the loop (fetch, diff,
PR) is in the client-side workflow.

## Normalization

The client workflow runs `jq -S .` on the fetched index before
committing. Keeps the file deterministic so future diffs only surface
real changes, not whitespace wobble. The first run after landing this
workflow will produce a large, one-time normalization diff; that's
expected and safe to merge.
