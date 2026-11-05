package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	TimeMS  int64  `json:"time_ms"`
	Time    string `json:"time"`
}

const landingPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#0d1317">
<title>fs-relay</title>
<style>
  :root {
    color-scheme: dark;
    --canvas: #0d1317;
    --raised: #161e23;
    --sunken: #080d10;
    --border: #232e34;
    --text-primary: #eef2f4;
    --text-secondary: #c3ced4;
    --text-tertiary: #8b98a1;
    --accent: #f43f5e;
    --accent-soft: #fda4af;
    --accent-deep: #be123c;
    --accent-wash: #2b1119;
    --font-display: system-ui, "Segoe UI", "Avenir Next", sans-serif;
    --font-body: Charter, "Bitstream Charter", "Sitka Text", Cambria, Georgia, serif;
    --font-mono: ui-monospace, "SF Mono", "Cascadia Mono", Menlo, Consolas, monospace;
  }
  * { box-sizing: border-box; }
  html {
    scrollbar-color: var(--accent-deep) var(--canvas);
    -webkit-font-smoothing: antialiased;
  }
  body {
    margin: 0;
    min-height: 100vh;
    display: grid;
    align-items: center;
    background:
      radial-gradient(ellipse 70% 35% at 8% 0%, rgb(244 63 94 / 0.10) 0%, transparent 58%),
      var(--canvas);
    color: var(--text-secondary);
    font-family: var(--font-body);
    font-size: 1.125rem;
    line-height: 1.65;
  }
  ::selection { background: var(--accent); color: #160509; }
  :focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
  main {
    padding: clamp(2rem, 6vh, 4rem) clamp(1.25rem, 5vw, 4rem);
    max-width: 48rem;
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-family: var(--font-mono);
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--accent-soft);
    background: var(--accent-wash);
    border: 1px solid var(--accent-deep);
    border-radius: 9999px;
    padding: 0.2rem 0.65rem;
    margin-bottom: 1rem;
  }
  .badge::before {
    content: "";
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 50%;
    background: var(--accent);
  }
  h1 {
    margin: 0;
    max-width: 16ch;
    font-family: var(--font-display);
    font-size: clamp(2.5rem, 6vw, 4rem);
    font-weight: 700;
    line-height: 1.05;
    letter-spacing: -0.035em;
    color: var(--text-primary);
    text-wrap: balance;
  }
  h1::after {
    content: "";
    display: block;
    width: 4.5rem;
    height: 3px;
    margin-top: 1.25rem;
    background: var(--accent);
    transform-origin: left center;
    animation: rule 0.22s cubic-bezier(0.22, 1, 0.36, 1) both;
  }
  p {
    margin: 1.25rem 0 0;
    max-width: 52ch;
    color: var(--text-secondary);
  }
  pre {
    margin: 1.25rem 0 0;
    padding: 0.9rem 1.15rem;
    background: var(--sunken);
    border: 1px solid var(--border);
    border-radius: 0.625rem;
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.06);
    font-family: var(--font-mono);
    font-size: 0.875rem;
    line-height: 1.5;
    color: var(--text-primary);
    overflow-x: auto;
  }
  @keyframes rule { from { transform: scaleX(0); } to { transform: scaleX(1); } }
  @media (prefers-reduced-motion: reduce) {
    h1::after { animation: none; }
  }
</style>
</head>
<body>
<main>
  <div class="badge">status: ok</div>
  <h1>Welcome to fs-relay server</h1>
  <p>A fast network filesystem over HTTP — about 10x faster than s3fs and NFS on real directory trees, with no root required.</p>
  <p>Mount a namespace on any Linux host with fs-relay-bind and pass the server's token with -token or -token-file:</p>
  <pre>fs-relay-bind -url http://127.0.0.1:8080 -ns default -token-file ~/.config/fs-relay/token /mnt/fs-relay</pre>
</main>
</body>
</html>
`

func serveRoot(w http.ResponseWriter, r *http.Request) {
	if preferJSON(r.Header.Get("Accept")) {
		writeHealth(w, time.Now().UTC())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(landingPage))
}

func writeHealth(w http.ResponseWriter, now time.Time) {
	body, err := json.Marshal(health{
		Status:  "ok",
		Version: buildVersion(),
		TimeMS:  now.UnixMilli(),
		Time:    now.Format(time.RFC3339),
	})
	if err != nil {
		http.Error(w, "health", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// preferJSON reports a health check. A missing Accept, "*", and "*/*" name no type.
// JSON is also the answer when application/json outranks text/html.
func preferJSON(accept string) bool {
	jsonQ, htmlQ := -1.0, -1.0
	for _, part := range strings.Split(accept, ",") {
		mt, q := mediaQ(part)
		if q <= 0 || mt == "*" || mt == "*/*" {
			continue
		}
		switch {
		case mt == "application/json" || mt == "text/json" || strings.HasSuffix(mt, "+json"):
			jsonQ = max(jsonQ, q)
		case mt == "text/html" || mt == "application/xhtml+xml":
			htmlQ = max(htmlQ, q)
		}
	}
	if htmlQ < 0 {
		return true
	}
	return jsonQ > htmlQ
}

func mediaQ(part string) (string, float64) {
	part = strings.TrimSpace(part)
	mt, rest, _ := strings.Cut(part, ";")
	q := 1.0
	for rest != "" {
		var item string
		item, rest, _ = strings.Cut(rest, ";")
		item = strings.TrimSpace(item)
		if !strings.HasPrefix(strings.ToLower(item), "q=") {
			continue
		}
		if v, err := strconv.ParseFloat(item[2:], 64); err == nil {
			q = v
		}
	}
	return strings.ToLower(strings.TrimSpace(mt)), q
}
