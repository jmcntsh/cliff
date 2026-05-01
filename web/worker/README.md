# cliff.sh Worker

The Cloudflare Worker behind `cliff.sh`. Three responsibilities:

1. `curl cliff.sh | sh` returns the install script as `text/plain`.
2. A browser visiting `https://cliff.sh/` gets a small landing page.
3. `cliff.sh/r/readme/<owner>/<repo>` and `cliff.sh/r/reel/<slug>`
   redirect to the upstream readme/reel and log one Analytics Engine
   data point per hit (used by the daily aggregator cron). The cliff
   client routes its readme and reel fetches through these so we have
   a credible "views per app" signal without a client-side telemetry
   endpoint or any account system.

Content negotiation (Accept header) decides between (1) and (2).
`/install.sh` permanently redirects to `/` so both URLs work.

The install script itself lives at
[`scripts/install.sh`](../../scripts/install.sh) in this repo. The Worker
fetches it from the GitHub raw URL on cache miss and caches the
response at the Cloudflare edge for 5 minutes. **Updating the script is
just `git push`** — no Worker redeploy needed.

## What gets logged

Each `/r/*` request writes one Analytics Engine data point:

| field | source |
| --- | --- |
| `kind` | `"readme"` or `"reel"` |
| `key`  | `<owner>/<repo>` for readme, `<slug>` for reel |
| `country` | Cloudflare's `cf.country` (ISO-3166 alpha-2) |
| `ipHash` | HMAC-SHA256(daily-rotating salt, `CF-Connecting-IP`), truncated to 16 hex chars |
| `uaHash` | HMAC-SHA256(daily-rotating salt, `User-Agent`), truncated to 16 hex chars |

The salt rotates every UTC day, derived from `CLIFF_TRACK_SECRET` +
the date string. Hashes are valid for distinct-counting within a day
and unlinkable across days. Raw IPs and UAs are never persisted.

Posture: this is a Cloudflare access log with extra structure, not
telemetry. There is no client-side opt-out flag because there is no
client-side beacon — opting out is "set `CLIFF_REGISTRY_URL` to
something else" or "don't run cliff."

## One-time setup

1. Add `cliff.sh` to your Cloudflare account (Add Site → free plan is
   fine). Change nameservers at the registrar.
2. Wait for DNS propagation. Cloudflare's dashboard will show "Active."
3. Install [wrangler](https://developers.cloudflare.com/workers/wrangler/install-and-update/):
   ```sh
   npm install -g wrangler
   wrangler login
   ```
4. Create the private R2 bucket for aggregated stats:
   ```sh
   wrangler r2 bucket create cliff-stats
   ```
5. Fill in `CF_ACCOUNT_ID` in `wrangler.toml` (Cloudflare dashboard →
   right sidebar of any account-scoped page).
6. Set the secrets:
   ```sh
   # 32+ bytes of random hex, e.g. `openssl rand -hex 32`
   wrangler secret put CLIFF_TRACK_SECRET
   # API token with "Account Analytics: Read" on this account
   wrangler secret put CF_API_TOKEN
   ```
7. Deploy:
   ```sh
   cd web/worker
   wrangler deploy
   ```

The Analytics Engine dataset (`cliff_events_v1`) is created on first
write — no separate provisioning step. The cron is registered on
deploy.

## Local dev

```sh
cd web/worker
wrangler dev
```

Opens a local server (default `http://localhost:8787`). Verify both
content-negotiated paths and a tracking redirect:

```sh
curl localhost:8787                                  # install script
curl -H 'Accept: text/html' localhost:8787 | head    # landing page
curl -I localhost:8787/r/reel/lazygit                # 302 to registry
curl -I localhost:8787/r/readme/jesseduffield/lazygit  # 302 to api.github
```

In `wrangler dev`, Analytics Engine writes are accepted but not
queryable via the SQL API; the cron handler will be a no-op locally
unless you wire a real `CF_API_TOKEN` (don't bother for normal dev).

## Inspecting collected data

The aggregator writes one JSON blob per UTC day to the private
`cliff-stats` bucket. To read yesterday's:

```sh
day=$(date -u -v-1d +%F 2>/dev/null || date -u -d 'yesterday' +%F)
wrangler r2 object get cliff-stats daily/$day.json
```

Schema (`schema_version: 1`):

```json
{
  "day": "2026-04-27",
  "generated_at": "2026-04-28T00:05:32.000Z",
  "schema_version": 1,
  "rows": [
    { "kind": "readme", "key": "jesseduffield/lazygit",
      "hits": 142, "distinct_ips": 118, "distinct_uas": 41 },
    { "kind": "reel", "key": "btop",
      "hits": 89,  "distinct_ips": 76,  "distinct_uas": 23 }
  ]
}
```

For ad-hoc queries against raw events (last 24h, etc.), use the
Analytics Engine SQL API directly with `CF_API_TOKEN`:

```sh
curl -X POST \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  https://api.cloudflare.com/client/v4/accounts/$CF_ACCOUNT_ID/analytics_engine/sql \
  --data 'SELECT blob1, blob2, count() FROM cliff_events_v1 WHERE timestamp >= now() - INTERVAL 1 DAY GROUP BY blob1, blob2 ORDER BY count() DESC LIMIT 50 FORMAT JSON'
```

## Updating

- **Install script content:** edit `scripts/install.sh`, push to main.
  Edge cache flushes after `CACHE_TTL_SECONDS` (5 min). To force-purge
  before then: Cloudflare dashboard → Caching → Purge URL →
  `https://cliff.sh/`.
- **Worker behavior:** edit `src/index.js`, run `wrangler deploy`.
- **Landing page:** edit `LANDING_HTML` in `src/index.js`, run
  `wrangler deploy`.
- **Aggregator schema:** bump `schema_version` in `aggregateYesterday`
  if the per-row shape changes; readers that key on the field will
  notice.
