// cliff.sh — Cloudflare Worker.
//
// Responsibilities:
//   1. `curl cliff.sh | sh` returns the install script as text/plain.
//   2. A browser visiting https://cliff.sh/ gets a tiny landing page
//      pointing at the GitHub repo and showing the one-line install.
//   3. /install.sh is a permanent redirect to / (one canonical URL).
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
    }
    a { color: #8a4d8a; text-decoration: none; }
    a:hover { text-decoration: underline; }
    footer { margin-top: 3rem; color: #888; font-size: 0.85rem; }
  </style>
</head>
<body>
  <h1>cliff</h1>
  <p class="tagline">A terminal-native directory for CLIs and TUIs.</p>

  <p>Install:</p>
  <pre>curl -fsSL https://cliff.sh | sh</pre>

  <p>Then run <code>cliff</code>. Press <code>?</code> for keybinds.</p>

  <p>
    <a href="https://github.com/jmcntsh/cliff">github.com/jmcntsh/cliff</a>
  </p>

  <footer>
    No accounts, no payments, no hosted binaries — cliff wraps brew, cargo,
    npm, pipx, and friends.
  </footer>
</body>
</html>
`;

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

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
};

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
