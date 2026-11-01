package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type health struct {
	Status string `json:"status"`
	TimeMS int64  `json:"time_ms"`
	Time   string `json:"time"`
}

const landingPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>fs-relay</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  html { scrollbar-color: #8a5a3a #14110e; }
  body {
    margin: 0;
    min-height: 100vh;
    display: grid;
    align-items: end;
    background:
      radial-gradient(ellipse 70% 45% at 8% -10%, #4a2a18 0%, transparent 58%),
      #14110e;
    color: #f4ecdf;
    font-family: "Iowan Old Style", "Palatino Linotype", Palatino, Georgia, serif;
  }
  ::selection { background: #e07a3d; color: #1a120c; }
  main {
    padding: clamp(2.5rem, 8vh, 6rem) clamp(1.5rem, 6vw, 7rem) clamp(3rem, 10vh, 7rem);
    max-width: 46rem;
  }
  h1 {
    margin: 0;
    max-width: 11ch;
    font-size: clamp(3.4rem, 9vw, 6rem);
    font-weight: 560;
    line-height: 0.9;
    letter-spacing: -0.035em;
    text-wrap: balance;
  }
  h1::after {
    content: "";
    display: block;
    width: 4.75rem;
    height: 3px;
    margin-top: 1.4rem;
    background: #e07a3d;
    transform-origin: left center;
    animation: rule 0.7s cubic-bezier(0.16, 1, 0.3, 1) both;
  }
  p {
    margin: 1.75rem 0 0;
    max-width: 34ch;
    font-family: "Avenir Next", "Segoe UI", sans-serif;
    font-size: 1.125rem;
    line-height: 1.45;
    color: #f0d7c4;
  }
  :focus-visible { outline: 2px solid #e07a3d; outline-offset: 3px; }
  @keyframes rule { from { transform: scaleX(0); } to { transform: scaleX(1); } }
  @media (prefers-reduced-motion: reduce) {
    h1::after { animation: none; }
  }
</style>
</head>
<body>
<main>
  <h1>Welcome to fs-relay server</h1>
  <p>A fast network filesystem. Mount it with fs-relay-bind.</p>
  <p>Pass the server's token to the mount with -token.</p>
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
		Status: "ok",
		TimeMS: now.UnixMilli(),
		Time:   now.Format(time.RFC3339),
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
