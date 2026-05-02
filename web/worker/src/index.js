// cliff.sh — Cloudflare Worker.
//
// Responsibilities:
//   1. `curl cliff.sh | sh` returns the install script as text/plain.
//   2. A browser visiting https://cliff.sh/ gets a tiny landing page
//      pointing at the GitHub repo and showing the one-line install.
//   3. /install.sh is a permanent redirect to / (one canonical URL).
//   4. /r/readme/<owner>/<repo> 302s to the GitHub readme API URL.
//   5. /r/reel/<slug>           302s to registry.cliff.sh/reels/<slug>.reel.
//
// (4) and (5) exist only so cliff fetches pass through a Worker we
// control, which lets us count them as Analytics Engine data points.
// We never see request bodies; the redirector only logs request
// metadata (path, slug, daily-rotated IP/UA hashes, country) and
// hands the client to the upstream URL. No client telemetry endpoint,
// no consent prompt — same posture as any CDN access log.
//
// The install script is fetched from the cliff repo's main branch on
// GitHub and cached at the Cloudflare edge. Updating the script is
// just `git push`; no Worker redeploy needed unless this file changes.
//
// To deploy:
//   cd web/worker && wrangler deploy
// Configure the cliff.sh route in wrangler.toml or the Cloudflare
// dashboard (Workers Routes → cliff.sh/* → this Worker).

const INSTALL_SH_URL =
  "https://raw.githubusercontent.com/jmcntsh/cliff/main/scripts/install.sh";

const CACHE_TTL_SECONDS = 300; // 5 minutes; balance freshness vs. origin load

// Tracking redirect targets. We redirect rather than proxy so the
// upstream (GitHub) sees the user's IP for its own rate-limit and so
// we don't pay bandwidth for pass-through bytes.
const README_UPSTREAM = (owner, repo) =>
  `https://api.github.com/repos/${owner}/${repo}/readme`;
const REEL_UPSTREAM = (slug) =>
  `https://registry.cliff.sh/reels/${slug}.reel`;

// Slug / owner / repo charset. Mirrors the cliff-registry lint rule
// (`[a-z0-9-]+` for slugs; owners and repos are GitHub's allowed set).
const SLUG_RE = /^[a-z0-9][a-z0-9-]{0,63}$/i;
const OWNER_RE = /^[a-z0-9][a-z0-9-]{0,38}$/i;
const REPO_RE = /^[a-z0-9._-]{1,100}$/i;

const LANDING_HTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>cliff — a terminal-native directory for CLIs and TUIs</title>
  <meta name="description" content="Browse, preview, and install terminal apps without leaving the terminal.">
  <style>
    :root { color-scheme: light dark; }
    body {
      font: 16px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace;
      max-width: 42rem;
      margin: 4rem auto;
      padding: 0 1.5rem;
      color: #1a1a1a;
      background: #fafafa;
    }
    @media (prefers-color-scheme: dark) {
      body { color: #e6e6e6; background: #111; }
      pre { background: #1c1c1c !important; }
      a { color: #c586c0; }
    }
    h1 { font-size: 1.6rem; margin-bottom: 0.2rem; }
    .tagline { color: #888; margin-top: 0; }
    pre {
      background: #f0f0f0;
      padding: 0.9rem 1rem;
      border-radius: 6px;
      overflow-x: auto;
      font-size: 0.95rem;
      margin: 0;
    }
    .cmd {
      position: relative;
      margin: 1rem 0;
    }
    .cmd pre { padding-right: 2.75rem; }
    .copy-btn {
      position: absolute;
      top: 0.45rem;
      right: 0.45rem;
      width: 2rem;
      height: 2rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      background: transparent;
      border: 1px solid transparent;
      border-radius: 4px;
      color: #888;
      cursor: pointer;
      font: inherit;
      padding: 0;
      transition: background 0.15s, color 0.15s, border-color 0.15s;
    }
    .copy-btn:hover {
      background: rgba(0, 0, 0, 0.06);
      color: #1a1a1a;
      border-color: rgba(0, 0, 0, 0.1);
    }
    .copy-btn:focus-visible {
      outline: 2px solid #8a4d8a;
      outline-offset: 1px;
    }
    .copy-btn svg { width: 1rem; height: 1rem; display: block; }
    .copy-btn.copied { color: #2e7d32; }
    @media (prefers-color-scheme: dark) {
      .copy-btn { color: #888; }
      .copy-btn:hover {
        background: rgba(255, 255, 255, 0.08);
        color: #e6e6e6;
        border-color: rgba(255, 255, 255, 0.12);
      }
      .copy-btn.copied { color: #81c784; }
    }
    a { color: #8a4d8a; text-decoration: none; }
    a:hover { text-decoration: underline; }
    footer { margin-top: 3rem; color: #888; font-size: 0.85rem; }
  </style>
</head>
<body>
  <h1>cliff</h1>
  <p class="tagline">A terminal-native directory for CLIs and TUIs.</p>

  <p>Install with Homebrew:</p>
  <div class="cmd">
    <pre id="cmd-brew">brew install jmcntsh/tap/cliff</pre>
    <button type="button" class="copy-btn" data-copy-target="cmd-brew" aria-label="Copy command">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <rect x="5" y="5" width="8" height="9" rx="1.25"></rect>
        <path d="M3 11V3.25A1.25 1.25 0 0 1 4.25 2H10"></path>
      </svg>
    </button>
  </div>

  <p>Or via the install script:</p>
  <div class="cmd">
    <pre id="cmd-curl">curl -fsSL https://cliff.sh | sh</pre>
    <button type="button" class="copy-btn" data-copy-target="cmd-curl" aria-label="Copy command">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <rect x="5" y="5" width="8" height="9" rx="1.25"></rect>
        <path d="M3 11V3.25A1.25 1.25 0 0 1 4.25 2H10"></path>
      </svg>
    </button>
  </div>

  <p>Then run <code>cliff</code>. Press <code>?</code> for keybinds.</p>

  <p>
    <a href="https://github.com/jmcntsh/cliff">github.com/jmcntsh/cliff</a>
  </p>

  <footer>
    No accounts, no payments, no hosted binaries — cliff wraps brew, cargo,
    npm, pipx, and friends.
  </footer>

  <script>
    (function () {
      var checkSVG =
        '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">' +
        '<path d="M3.5 8.5l3 3 6-7"></path></svg>';
      var copySVG =
        '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">' +
        '<rect x="5" y="5" width="8" height="9" rx="1.25"></rect>' +
        '<path d="M3 11V3.25A1.25 1.25 0 0 1 4.25 2H10"></path></svg>';

      function fallbackCopy(text) {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.setAttribute("readonly", "");
        ta.style.position = "absolute";
        ta.style.left = "-9999px";
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); } catch (e) {}
        document.body.removeChild(ta);
      }

      document.querySelectorAll(".copy-btn").forEach(function (btn) {
        btn.addEventListener("click", function () {
          var target = document.getElementById(btn.dataset.copyTarget);
          if (!target) return;
          var text = target.textContent.trim();
          var done = function () {
            btn.classList.add("copied");
            btn.setAttribute("aria-label", "Copied");
            btn.innerHTML = checkSVG;
            setTimeout(function () {
              btn.classList.remove("copied");
              btn.setAttribute("aria-label", "Copy command");
              btn.innerHTML = copySVG;
            }, 1500);
          };
          if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(done, function () {
              fallbackCopy(text);
              done();
            });
          } else {
            fallbackCopy(text);
            done();
          }
        });
      });
    })();
  </script>
</body>
</html>
`;

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    // Public hot-score sidecar. Served from cliff.sh/hot.json so we
    // don't need a second hostname or its DNS work; the client knows
    // to fetch this exact path. Reads from the (otherwise private)
    // cliff-stats R2 bucket. 404 is the expected response while the
    // aggregator is still gated on minimum days-seen — clients
    // tolerate it (see internal/hotfetch).
    if (url.pathname === "/hot.json") {
      return await serveHotJSON(env);
    }

    // Tracking redirectors. Match before the canonical / 404 paths so
    // a future "/r/" route can't be shadowed by /install.sh-style
    // rewrites. trackEvent is best-effort: a logging failure must not
    // turn a real fetch into a 500.
    if (url.pathname.startsWith("/r/")) {
      return await handleRedirect(url, request, env, ctx);
    }

    // Canonicalize: /install.sh → /
    if (url.pathname === "/install.sh") {
      return Response.redirect(url.origin + "/", 301);
    }

    if (url.pathname !== "/" && url.pathname !== "") {
      return new Response("not found\n", {
        status: 404,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    }

    // Content negotiation: browsers want HTML, curl wants the script.
    // Heuristic: explicit Accept: text/html → HTML. Anything else (curl
    // sends Accept: */* by default) → script. This matches what
    // sh.rustup.rs and get.docker.com do.
    const accept = request.headers.get("Accept") || "";
    const wantsHTML = accept.includes("text/html");

    if (wantsHTML) {
      return new Response(LANDING_HTML, {
        headers: {
          "content-type": "text/html; charset=utf-8",
          "cache-control": `public, max-age=${CACHE_TTL_SECONDS}`,
        },
      });
    }

    return await serveInstallScript(ctx);
  },

  // Daily aggregator. Reads the previous UTC day's data points from
  // Analytics Engine via the SQL API and writes a per-day stats.json
  // to the (private) STATS R2 bucket. Then computes a recency-weighted
  // "hot score" over a 21-day window and writes that to the same
  // bucket — gated on having seen at least HOT_MIN_DAYS_SEEN distinct
  // days of data, so the first emit isn't built on a partial window.
  // Surfacing on the client is gated separately (see
  // internal/hotfetch + the Hot sidebar reveal threshold).
  async scheduled(event, env, ctx) {
    ctx.waitUntil(aggregateYesterday(env));
    ctx.waitUntil(aggregateHot(env));
  },
};

// ---------- /r/* tracking redirectors -------------------------------

async function handleRedirect(url, request, env, ctx) {
  const parts = url.pathname.split("/").filter(Boolean); // ["r", kind, ...]
  const kind = parts[1];

  let target = null;
  let key = null; // slug or owner/repo, used as the analytics dimension

  if (kind === "readme" && parts.length === 4) {
    const owner = parts[2];
    const repo = parts[3];
    if (!OWNER_RE.test(owner) || !REPO_RE.test(repo)) {
      return badRedirect();
    }
    target = README_UPSTREAM(owner, repo);
    key = `${owner}/${repo}`;
  } else if (kind === "reel" && parts.length === 3) {
    const slug = parts[2];
    if (!SLUG_RE.test(slug)) return badRedirect();
    target = REEL_UPSTREAM(slug);
    key = slug;
  } else {
    return badRedirect();
  }

  // Log first, redirect second. The log call is async; we don't await
  // it on the hot path beyond what AE.writeDataPoint needs to enqueue.
  ctx.waitUntil(logEvent(env, kind, key, request));

  // 302, not 301: target URLs are not stable forever (we may rotate
  // upstreams later) and we don't want clients/CDNs caching the
  // redirect itself across a config change.
  return Response.redirect(target, 302);
}

function badRedirect() {
  return new Response("bad redirect path\n", {
    status: 404,
    headers: { "content-type": "text/plain; charset=utf-8" },
  });
}

// ---------- Analytics Engine logging --------------------------------

// We write one data point per redirect with a small set of dimensions.
// Identity is reduced to two HMAC-SHA256 hashes keyed on a salt that
// rotates every UTC day, so distinct IP/UA counts are valid within a
// day but cannot be linked across days. The salt itself is derived
// from a static Worker secret + the UTC date string — no per-day
// secret rotation needed.
async function logEvent(env, kind, key, request) {
  if (!env.CLIFF_EVENTS) return; // binding missing → silently skip

  const ip = request.headers.get("CF-Connecting-IP") || "";
  const ua = request.headers.get("User-Agent") || "";
  const country =
    (request.cf && request.cf.country) || ""; // "US", "DE", "T1" (Tor), etc.
  const day = utcDayString(new Date());

  const salt = await dailySalt(env, day);
  const ipHash = await hmacShort(salt, ip);
  const uaHash = await hmacShort(salt, ua);

  // AE schema:
  //   blobs[0] = kind ("readme" | "reel")
  //   blobs[1] = key  (owner/repo for readme; slug for reel)
  //   blobs[2] = country (ISO-3166 alpha-2 from Cloudflare)
  //   blobs[3] = ipHash (16 hex chars)
  //   blobs[4] = uaHash (16 hex chars)
  //   indexes[0] = key — primary high-cardinality grouping dimension
  //   doubles    — unused for v1
  try {
    env.CLIFF_EVENTS.writeDataPoint({
      blobs: [kind, key, country, ipHash, uaHash],
      indexes: [key],
    });
  } catch (_e) {
    // Never block on logging failures.
  }
}

async function dailySalt(env, day) {
  const baseSecret = (env.CLIFF_TRACK_SECRET || "cliff-dev-salt").toString();
  const enc = new TextEncoder();
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    enc.encode(baseSecret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", keyMaterial, enc.encode(day));
  return new Uint8Array(sig);
}

async function hmacShort(saltBytes, value) {
  if (!value) return "";
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    saltBytes,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sig = await crypto.subtle.sign(
    "HMAC",
    keyMaterial,
    new TextEncoder().encode(value),
  );
  // 8 bytes = 16 hex chars. Plenty for distinct-counting within a
  // single day's keyspace; not enough to mount any meaningful
  // pre-image against arbitrary IPs without the rotating salt.
  return bytesToHex(new Uint8Array(sig).subarray(0, 8));
}

function bytesToHex(bytes) {
  const hex = [];
  for (let i = 0; i < bytes.length; i++) {
    hex.push(bytes[i].toString(16).padStart(2, "0"));
  }
  return hex.join("");
}

function utcDayString(d) {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// ---------- Hot-score sidecar serve ---------------------------------

// serveHotJSON reads the latest hot.json from R2 and returns it with
// a short edge cache. Returns 404 (not 500) when no file exists yet,
// because the client treats 404 as "hot data not available; hide the
// surface" — same as the redirector-fallback contract elsewhere in
// this file. R2 reads are cheap; we don't bother with a Worker-side
// cache layer beyond the edge cache header.
async function serveHotJSON(env) {
  if (!env.STATS) {
    return new Response("stats bucket not bound\n", {
      status: 503,
      headers: { "content-type": "text/plain; charset=utf-8" },
    });
  }
  const obj = await env.STATS.get("hot.json");
  if (!obj) {
    return new Response("hot data not yet available\n", {
      status: 404,
      headers: { "content-type": "text/plain; charset=utf-8" },
    });
  }
  // 6h cache: hot.json regenerates daily at 00:05 UTC, so 6h means
  // a worst-case freshness lag of <quarter-day for clients that
  // happened to fetch right after the prior day's emit. ETag from
  // R2 lets clients revalidate cheaply on cache miss.
  const headers = new Headers();
  headers.set("content-type", "application/json; charset=utf-8");
  headers.set("cache-control", "public, max-age=21600");
  if (obj.etag) headers.set("etag", obj.etag);
  return new Response(obj.body, { headers });
}

// ---------- Hot-score aggregation cron ------------------------------

// HOT_HALF_LIFE_DAYS is the time it takes a single viewer-day to
// decay to half its weight. 7 days mirrors the user-locked design:
// short enough that "hot" responds to recent behavior, long enough
// that one viral Tuesday doesn't dominate Friday's ranking.
const HOT_HALF_LIFE_DAYS = 7;

// HOT_WINDOW_DAYS is how far back the aggregator queries. After
// ~3× half-life, a viewer-day's weight is ~12% — still measurable
// but small enough that extending the window further adds noise
// more than signal. 21 days = 3 half-lives.
const HOT_WINDOW_DAYS = 21;

// HOT_MIN_DAYS_SEEN gates the first emit. We won't publish hot.json
// at all until Analytics Engine has seen this many distinct UTC
// days of data — otherwise the first deploy publishes a real
// ranking based on whatever happened in the past few hours. Two
// weeks of data is the minimum that lets a viewer-day from the
// start of the window meaningfully decay against viewer-days at
// the end.
const HOT_MIN_DAYS_SEEN = 14;

// HOT_MIN_LIFETIME_IPS gates per-app emit. An app with very few
// lifetime distinct viewers shouldn't surface as "hot" no matter
// the decay arithmetic — its score is dominated by a single small
// cluster, not signal. 5 is generous enough that genuinely
// emerging tiny apps still appear once they have any traction at
// all, strict enough that a single curious cluster doesn't
// produce a ranked entry.
const HOT_MIN_LIFETIME_IPS = 5;

// aggregateHot computes a recency-weighted hot score per (kind, key)
// over the last HOT_WINDOW_DAYS, gates on minimum days-seen, and
// writes hot.json into R2. The score for one (kind, key) is:
//
//   sum over days d in window of:
//     distinct_ips(d) * exp(-(today - d) / HOT_HALF_LIFE_DAYS)
//
// Since AE's uniq() is per-day-bucketed here, an ip that appeared
// on multiple days contributes once per day. Same posture as the
// daily file's distinct_ips — we explicitly want "this person came
// back" to count more than "this person browsed once."
//
// Idempotent: re-running the same day overwrites hot.json with the
// same content modulo `generated_at`. Rerunnable mid-day if you
// want to test.
async function aggregateHot(env) {
  if (!env.STATS) return;
  if (!env.CF_ACCOUNT_ID || !env.CF_API_TOKEN || !env.CLIFF_EVENTS_DATASET) {
    console.log("aggregateHot: missing CF_ACCOUNT_ID / CF_API_TOKEN / CLIFF_EVENTS_DATASET");
    return;
  }

  const now = new Date();
  const today = utcDayString(now);

  // Per-day distinct-IP buckets across the window. Group by
  // (kind, key, day) so the JS layer can multiply each bucket by
  // its decay weight. We use toDate(timestamp) rather than
  // truncating in JS so a row that straddles midnight UTC counts
  // as two day-buckets, not one — matches how the daily file
  // partitions events.
  const sql = `
    SELECT
      blob1 AS kind,
      blob2 AS key,
      toDate(timestamp) AS day,
      uniq(blob4) AS distinct_ips
    FROM ${env.CLIFF_EVENTS_DATASET}
    WHERE timestamp >= now() - INTERVAL '${HOT_WINDOW_DAYS}' DAY
    GROUP BY kind, key, day
    FORMAT JSON
  `.trim();

  const resp = await fetch(
    `https://api.cloudflare.com/client/v4/accounts/${env.CF_ACCOUNT_ID}/analytics_engine/sql`,
    {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${env.CF_API_TOKEN}`,
        "Content-Type": "text/plain",
      },
      body: sql,
    },
  );

  if (!resp.ok) {
    console.log(`aggregateHot: AE query failed ${resp.status}`);
    return;
  }

  const result = await resp.json();
  const rows = (result && result.data) || [];
  if (rows.length === 0) {
    console.log("aggregateHot: no rows in window; skipping emit");
    return;
  }

  // Days-seen gate. Count distinct day buckets across the whole
  // result set. Below the threshold we don't publish at all — the
  // existing hot.json (if any) stays in place rather than being
  // overwritten with a thinner score, so a brief AE outage can't
  // wipe out a good ranking.
  const daysSeen = new Set(rows.map((r) => r.day)).size;
  if (daysSeen < HOT_MIN_DAYS_SEEN) {
    console.log(`aggregateHot: only ${daysSeen}/${HOT_MIN_DAYS_SEEN} days of data; skipping emit`);
    return;
  }

  // Bucket rows by (kind, key) and accumulate decayed score plus
  // raw lifetime distinct-IPs over the window. Lifetime here means
  // "across the queried window," which is fine for a 21-day window
  // — outside the window the events have decayed to ~0 anyway.
  const buckets = new Map(); // composite "kind:key" -> {kind, key, score, lifetime_ips}
  for (const r of rows) {
    const kind = r.kind;
    const key = r.key;
    if (!kind || !key) continue;
    const ips = Number(r.distinct_ips) || 0;
    if (ips <= 0) continue;
    const ageDays = daysBetween(today, r.day);
    if (ageDays < 0 || ageDays > HOT_WINDOW_DAYS) continue;
    const weight = Math.exp(-ageDays / HOT_HALF_LIFE_DAYS);
    const composite = `${kind}:${key}`;
    let b = buckets.get(composite);
    if (!b) {
      b = { kind, key, score: 0, lifetime_ips: 0 };
      buckets.set(composite, b);
    }
    b.score += ips * weight;
    b.lifetime_ips += ips;
  }

  // Per-app floor: drop entries with too few lifetime viewers to
  // be meaningful. Sort descending by score so the client can take
  // top-N without resorting if it ever wants to.
  const filtered = [];
  for (const b of buckets.values()) {
    if (b.lifetime_ips < HOT_MIN_LIFETIME_IPS) continue;
    filtered.push({
      kind: b.kind,
      key: b.key,
      hot_score: Number(b.score.toFixed(3)),
      lifetime_ips: b.lifetime_ips,
    });
  }
  filtered.sort((a, b) => b.hot_score - a.hot_score);

  const out = {
    generated_at: now.toISOString(),
    schema_version: 1,
    half_life_days: HOT_HALF_LIFE_DAYS,
    window_days: HOT_WINDOW_DAYS,
    days_seen: daysSeen,
    min_lifetime_ips: HOT_MIN_LIFETIME_IPS,
    rows: filtered,
  };

  await env.STATS.put(
    "hot.json",
    JSON.stringify(out, null, 2),
    { httpMetadata: { contentType: "application/json" } },
  );
  console.log(`aggregateHot: emitted ${filtered.length} rows over ${daysSeen} days`);
}

// daysBetween returns the integer count of UTC days between two
// "YYYY-MM-DD" strings, computed via Date.UTC so DST never enters
// the picture. Used by the hot aggregator to compute a row's
// recency in days for the decay weight.
function daysBetween(todayStr, dayStr) {
  const t = parseUTCDate(todayStr);
  const d = parseUTCDate(dayStr);
  if (t == null || d == null) return -1;
  return Math.round((t - d) / (24 * 60 * 60 * 1000));
}

function parseUTCDate(s) {
  if (typeof s !== "string") return null;
  const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!m) return null;
  return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

// ---------- Daily aggregation cron ----------------------------------

// aggregateYesterday queries Analytics Engine via the SQL API for the
// previous UTC day's events, computes per-key event counts and
// distinct-IP/UA counts, and writes a stats.json into the (private)
// STATS R2 bucket. Idempotent: re-running for the same day overwrites
// the same object.
async function aggregateYesterday(env) {
  if (!env.STATS) return; // bucket binding missing → nothing to write
  if (!env.CF_ACCOUNT_ID || !env.CF_API_TOKEN || !env.CLIFF_EVENTS_DATASET) {
    // No way to query AE without these; skip and log on next deploy.
    console.log("aggregateYesterday: missing CF_ACCOUNT_ID / CF_API_TOKEN / CLIFF_EVENTS_DATASET");
    return;
  }

  const now = new Date();
  const yest = new Date(now.getTime() - 24 * 60 * 60 * 1000);
  const day = utcDayString(yest);

  // SQL via Workers Analytics Engine. blob1 = kind, blob2 = key,
  // blob3 = country, blob4 = ipHash, blob5 = uaHash. Distinct counts
  // use uniq(...) which is approximate (HyperLogLog) but cheap and
  // good enough for the granularity we surface.
  const sql = `
    SELECT
      blob1 AS kind,
      blob2 AS key,
      count() AS hits,
      uniq(blob4) AS distinct_ips,
      uniq(blob5) AS distinct_uas
    FROM ${env.CLIFF_EVENTS_DATASET}
    WHERE timestamp >= toDateTime('${day} 00:00:00')
      AND timestamp <  toDateTime('${day} 00:00:00') + INTERVAL '1' DAY
    GROUP BY kind, key
    ORDER BY hits DESC
    FORMAT JSON
  `.trim();

  const resp = await fetch(
    `https://api.cloudflare.com/client/v4/accounts/${env.CF_ACCOUNT_ID}/analytics_engine/sql`,
    {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${env.CF_API_TOKEN}`,
        "Content-Type": "text/plain",
      },
      body: sql,
    },
  );

  if (!resp.ok) {
    console.log(`aggregateYesterday: AE query failed ${resp.status}`);
    return;
  }

  const result = await resp.json();
  const rows = (result && result.data) || [];

  const stats = {
    day,
    generated_at: now.toISOString(),
    schema_version: 1,
    rows: rows.map((r) => ({
      kind: r.kind,
      key: r.key,
      hits: Number(r.hits) || 0,
      distinct_ips: Number(r.distinct_ips) || 0,
      distinct_uas: Number(r.distinct_uas) || 0,
    })),
  };

  await env.STATS.put(
    `daily/${day}.json`,
    JSON.stringify(stats, null, 2),
    {
      httpMetadata: { contentType: "application/json" },
    },
  );
}

// ---------- Existing install-script + landing page handlers ---------

async function serveInstallScript(ctx) {
  const cache = caches.default;
  const cacheKey = new Request(INSTALL_SH_URL, { method: "GET" });

  let cached = await cache.match(cacheKey);
  if (cached) return rewriteHeaders(cached);

  const upstream = await fetch(INSTALL_SH_URL, {
    cf: { cacheTtl: CACHE_TTL_SECONDS, cacheEverything: true },
  });
  if (!upstream.ok) {
    return new Response(
      `# cliff installer fetch failed (${upstream.status})\n` +
        `# please report at https://github.com/jmcntsh/cliff/issues\n` +
        `exit 1\n`,
      {
        status: 502,
        headers: { "content-type": "text/plain; charset=utf-8" },
      },
    );
  }

  const body = await upstream.text();
  const response = new Response(body, {
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": `public, max-age=${CACHE_TTL_SECONDS}`,
      "x-cliff-source": INSTALL_SH_URL,
    },
  });

  ctx.waitUntil(cache.put(cacheKey, response.clone()));
  return response;
}

function rewriteHeaders(response) {
  // Cached response keeps content-type from upstream (raw.githubusercontent
  // serves text/plain, which is what we want — but be defensive).
  const headers = new Headers(response.headers);
  headers.set("content-type", "text/plain; charset=utf-8");
  return new Response(response.body, {
    status: response.status,
    headers,
  });
}
