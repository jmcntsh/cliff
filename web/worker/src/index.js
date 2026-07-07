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

// ---------- Install-script + landing page handlers ------------------

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
