# cliff.sh Worker

The Cloudflare Worker behind `cliff.sh`. Two responsibilities:

1. `curl cliff.sh | sh` returns the install script as `text/plain`.
2. A browser visiting `https://cliff.sh/` gets a small landing page.

Content negotiation (Accept header) decides which one. `/install.sh`
permanently redirects to `/` so both URLs work.

The install script itself lives at
[`scripts/install.sh`](../../scripts/install.sh) in this repo. The Worker
fetches it from the GitHub raw URL on cache miss and caches the
response at the Cloudflare edge for 5 minutes. **Updating the script is
just `git push`** — no Worker redeploy needed.

## One-time setup

1. Add `cliff.sh` to your Cloudflare account (Add Site → free plan is
   fine). Change nameservers at the registrar.
2. Wait for DNS propagation. Cloudflare's dashboard will show "Active."
3. Install [wrangler](https://developers.cloudflare.com/workers/wrangler/install-and-update/):
   ```sh
   npm install -g wrangler
   wrangler login
   ```
4. Uncomment the `routes` block in `wrangler.toml`.
5. Deploy:
   ```sh
   cd web/worker
   wrangler deploy
   ```

That's it. `curl cliff.sh` should return the install script.

## Local dev

```sh
cd web/worker
wrangler dev
```

Opens a local server (default `http://localhost:8787`). Hit it with both
`curl localhost:8787` and a browser to verify both code paths.

## Updating

- **Install script content:** edit `scripts/install.sh`, push to main.
  Edge cache flushes after `CACHE_TTL_SECONDS` (5 min). To force-purge
  before then: Cloudflare dashboard → Caching → Purge URL →
  `https://cliff.sh/`.
- **Worker behavior:** edit `src/index.js`, run `wrangler deploy`.
- **Landing page:** edit `LANDING_HTML` in `src/index.js`, run
  `wrangler deploy`.

## Future hooks

If/when an install ping ships, add a D1 database binding here and a
`POST /events` route in `src/index.js`. That keeps cliff's whole infra
footprint inside one Cloudflare account.
