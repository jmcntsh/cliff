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
  <meta name="theme-color" content="#050705">
  <link rel="preload" href="https://jmcnt.sh/fonts/DepartureMono-Regular.woff2" as="font" type="font/woff2" crossorigin>
  <style>
    /* Matches the jmcnt.sh HUD look: green Departure Mono over a darkened nature video */
    @font-face {
      font-family: "Departure Mono";
      src: url("https://jmcnt.sh/fonts/DepartureMono-Regular.woff2") format("woff2"),
           url("https://jmcnt.sh/fonts/DepartureMono-Regular.woff") format("woff");
      font-display: swap;
    }
    :root {
      color-scheme: dark;
      --hud: #6dff9a;
      --hud-dim: rgba(109, 255, 154, 0.62);
      --line: rgba(109, 255, 154, 0.35);
      --glow:
        0 0 2px rgba(109, 255, 154, 0.9),
        0 0 10px rgba(109, 255, 154, 0.45),
        0 0 28px rgba(109, 255, 154, 0.25);
      --glow-soft: 0 0 6px rgba(109, 255, 154, 0.35);
    }
    html { background: #050705; }
    /* Departure Mono is drawn on an 11px grid; sizes stay on multiples of 11 */
    body {
      max-width: 44rem;
      margin: 0 auto;
      padding: 7rem 1.5rem 4rem;
      background: transparent;
      color: var(--hud);
      font: 11px/1.9 "Departure Mono", ui-monospace, Menlo, monospace;
      letter-spacing: 0.06em;
      text-shadow: var(--glow-soft);
      -webkit-font-smoothing: antialiased;
    }
    .bg {
      position: fixed;
      inset: 0;
      z-index: -1;
      background: #050705 center / cover no-repeat;
    }
    .bg video { width: 100%; height: 100%; object-fit: cover; }
    .bg::after {
      content: "";
      position: absolute;
      inset: 0;
      background:
        radial-gradient(ellipse at 50% 40%, rgba(4, 7, 4, 0.5) 0%, rgba(4, 7, 4, 0.78) 100%),
        rgba(4, 7, 4, 0.35);
    }
    body::after {
      content: "";
      position: fixed;
      inset: 0;
      z-index: 2000;
      pointer-events: none;
      background: repeating-linear-gradient(to bottom, transparent 0 2px, rgba(0, 0, 0, 0.22) 2px 3px);
    }
    h1 {
      margin: 0 0 0.4rem;
      font-size: 88px;
      font-weight: 400;
      line-height: 1;
      text-shadow: var(--glow);
    }
    h1::after {
      content: "";
      display: inline-block;
      width: 0.5em;
      height: 0.8em;
      margin-left: 0.1em;
      background: var(--hud);
      box-shadow: 0 0 8px var(--hud-dim);
      animation: blink 1s steps(1) infinite;
    }
    @keyframes blink { 50% { opacity: 0; } }
    .tagline {
      margin: 0 0 3rem;
      font-size: 22px;
      line-height: 1.4;
      text-transform: uppercase;
      text-shadow: var(--glow);
    }
    p { color: var(--hud-dim); }
    code { color: var(--hud); font: inherit; }
    pre {
      margin: 0;
      padding: 0.9rem 1rem;
      overflow-x: auto;
      border: 1px solid var(--line);
      font: inherit;
      font-size: 22px;
      line-height: 1.4;
      color: var(--hud);
      text-shadow: var(--glow);
    }
    pre::before { content: "$ "; color: var(--hud-dim); }
    .cmd { position: relative; margin: 1rem 0 2rem; }
    .cmd pre { padding-right: 3.5rem; }
    .copy-btn {
      position: absolute;
      top: 50%;
      right: 0.6rem;
      width: 2.2rem;
      height: 2.2rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      transform: translateY(-50%);
      background: transparent;
      border: 1px solid transparent;
      color: var(--hud-dim);
      cursor: pointer;
      padding: 0;
    }
    .copy-btn:hover { color: var(--hud); border-color: var(--line); background: rgba(109, 255, 154, 0.08); }
    .copy-btn:focus-visible { outline: 1px dashed var(--hud-dim); outline-offset: 2px; }
    .copy-btn svg { width: 1.1rem; height: 1.1rem; display: block; }
    .copy-btn.copied { color: var(--hud); }
    a { color: var(--hud); text-decoration: underline; text-underline-offset: 0.2em; }
    a:hover { text-shadow: var(--glow); }
    footer { margin-top: 3rem; color: var(--hud-dim); }
    ::selection { background: rgba(109, 255, 154, 0.3); color: var(--hud); }
    @media (max-width: 720px) {
      body { padding-top: 4.5rem; }
      h1 { font-size: 66px; }
      pre { font-size: 11px; }
    }
    @media (prefers-reduced-motion: reduce) {
      h1::after { animation: none; }
    }
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
        <rect x="5" y="5" width="8" height="9"></rect>
        <path d="M3 11V2h7"></path>
      </svg>
    </button>
  </div>

  <p>Or via the install script:</p>
  <div class="cmd">
    <pre id="cmd-curl">curl -fsSL https://cliff.sh | sh</pre>
    <button type="button" class="copy-btn" data-copy-target="cmd-curl" aria-label="Copy command">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <rect x="5" y="5" width="8" height="9"></rect>
        <path d="M3 11V2h7"></path>
      </svg>
    </button>
  </div>

  <p>Then run <code>cliff</code>. Press <code>?</code> for keybinds.</p>

  <p>
    <a href="https://github.com/jmcntsh/cliff">github.com/jmcntsh/cliff</a>
  </p>

  <footer>
    No accounts, no payments, no hosted binaries. cliff wraps brew, cargo,
    npm, pipx, and friends.
  </footer>

  <script>
    (function () {
      var clips = ["meadow", "sea", "drive"];
      var clip = clips[Math.floor(Math.random() * clips.length)];
      var base = "https://jmcnt.sh/media/" + clip;
      var bg = document.createElement("div");
      bg.className = "bg";
      bg.setAttribute("aria-hidden", "true");
      bg.style.backgroundImage = "url(" + base + "-poster.jpg)";
      var reduceMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;
      var saveData = navigator.connection && navigator.connection.saveData === true;
      if (!reduceMotion && !saveData) {
        var video = document.createElement("video");
        video.muted = true;
        video.loop = true;
        video.autoplay = true;
        video.playsInline = true;
        video.setAttribute("muted", "");
        video.setAttribute("playsinline", "");
        video.poster = base + "-poster.jpg";
        video.src = base + (matchMedia("(max-width: 720px)").matches ? "-720.mp4" : "-1080.mp4");
        bg.appendChild(video);
        video.play().catch(function () {});
      }
      document.body.prepend(bg);
    })();
  </script>
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
