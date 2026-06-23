package ratelimit

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
)

// Dashboard serves a read-only, auto-refreshing view of one or more
// Limiters' current state: every key each limiter is tracking, its
// remaining budget, and when its limit resets. It shows the live state at
// the time of each request, not a history of past requests.
//
// Dashboard is independent of any particular mux or port: use Handler to
// mount it within an existing server, or ListenAndServe to run it on its
// own address.
type Dashboard struct {
	names    []string
	limiters map[string]*Limiter
}

// NewDashboard creates a Dashboard showing the given limiters. The map
// keys are display names only (e.g. "hello", "strict") and don't need to
// match anything else in your application.
func NewDashboard(limiters map[string]*Limiter) *Dashboard {
	names := make([]string, 0, len(limiters))
	for name := range limiters {
		names = append(names, name)
	}
	sort.Strings(names)

	return &Dashboard{names: names, limiters: limiters}
}

// dashboardLimiterView is one limiter's data in the JSON snapshot.
type dashboardLimiterView struct {
	Name       string             `json:"name"`
	Algorithm  string             `json:"algorithm"`
	Rate       int                `json:"rate"`
	PerSeconds float64            `json:"per_seconds"`
	Enumerable bool               `json:"enumerable"`
	Keys       []dashboardKeyView `json:"keys"`
}

type dashboardKeyView struct {
	Key        string `json:"key"`
	Allowed    bool   `json:"allowed"`
	Remaining  int    `json:"remaining"`
	ResetAtRFC string `json:"reset_at"`
}

func (d *Dashboard) snapshotJSON() []dashboardLimiterView {
	views := make([]dashboardLimiterView, 0, len(d.names))
	for _, name := range d.names {
		l := d.limiters[name]
		view := dashboardLimiterView{
			Name:       name,
			Algorithm:  string(l.Algorithm()),
			Rate:       l.Rate(),
			PerSeconds: l.Per().Seconds(),
		}

		snap, ok := l.Snapshot()
		view.Enumerable = ok
		if ok {
			view.Keys = make([]dashboardKeyView, 0, len(snap))
			for _, ks := range snap {
				view.Keys = append(view.Keys, dashboardKeyView{
					Key:        ks.Key,
					Allowed:    ks.Result.Allowed,
					Remaining:  ks.Result.Remaining,
					ResetAtRFC: ks.Result.ResetAt.Format("15:04:05"),
				})
			}
			sort.Slice(view.Keys, func(i, j int) bool {
				return view.Keys[i].Key < view.Keys[j].Key
			})
		}

		views = append(views, view)
	}
	return views
}

// Handler returns an http.Handler serving the dashboard's HTML page at its
// root and a JSON snapshot at "/api/snapshot". Mount it under any prefix
// with http.StripPrefix if you don't want it at the root of its own mux.
func (d *Dashboard) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.serveHTML)
	mux.HandleFunc("/api/snapshot", d.serveSnapshot)
	return mux
}

// ListenAndServe runs the dashboard standalone on addr (e.g. ":9090"),
// independent of your application's main listener.
func (d *Dashboard) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, d.Handler())
}

func (d *Dashboard) serveSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.snapshotJSON())
}

func (d *Dashboard) serveHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	dashboardPage.Execute(w, nil)
}

var dashboardPage = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>go-rataliy_lib dashboard</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 2rem; color: #1a1a1a; }
  h1 { font-size: 1.25rem; }
  table { border-collapse: collapse; width: 100%; margin-bottom: 2rem; }
  th, td { text-align: left; padding: 0.4rem 0.75rem; border-bottom: 1px solid #ddd; font-size: 0.9rem; }
  th { color: #555; font-weight: 600; }
  .limiter-meta { color: #555; font-size: 0.85rem; margin-bottom: 0.5rem; }
  .allowed { color: #1a7f37; }
  .denied { color: #c1121f; font-weight: 600; }
  .empty, .unsupported { color: #777; font-style: italic; padding: 0.5rem 0; }
  .updated { color: #777; font-size: 0.8rem; }
</style>
</head>
<body>
<h1>go-rataliy_lib dashboard</h1>
<p class="updated">Last updated: <span id="updated-at">—</span> (refreshes every 2s)</p>
<div id="limiters"></div>

<script>
function render(data) {
  const root = document.getElementById('limiters');
  root.innerHTML = '';
  for (const limiter of data) {
    const section = document.createElement('section');

    const h2 = document.createElement('h2');
    h2.textContent = limiter.name;
    section.appendChild(h2);

    const meta = document.createElement('div');
    meta.className = 'limiter-meta';
    meta.textContent = limiter.algorithm + ' — ' + limiter.rate + ' req / ' + limiter.per_seconds + 's';
    section.appendChild(meta);

    if (!limiter.enumerable) {
      const p = document.createElement('p');
      p.className = 'unsupported';
      p.textContent = 'This limiter\'s store does not support listing keys.';
      section.appendChild(p);
    } else if (limiter.keys.length === 0) {
      const p = document.createElement('p');
      p.className = 'empty';
      p.textContent = 'No requests tracked yet.';
      section.appendChild(p);
    } else {
      const table = document.createElement('table');
      table.innerHTML = '<thead><tr><th>Key</th><th>Status</th><th>Remaining</th><th>Resets at</th></tr></thead>';
      const tbody = document.createElement('tbody');
      for (const k of limiter.keys) {
        const tr = document.createElement('tr');
        const statusClass = k.allowed ? 'allowed' : 'denied';
        const statusText = k.allowed ? 'ok' : 'limited';
        tr.innerHTML = '<td>' + escapeHTML(k.key) + '</td>' +
          '<td class="' + statusClass + '">' + statusText + '</td>' +
          '<td>' + k.remaining + '</td>' +
          '<td>' + k.reset_at + '</td>';
        tbody.appendChild(tr);
      }
      table.appendChild(tbody);
      section.appendChild(table);
    }

    root.appendChild(section);
  }
}

function escapeHTML(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

function refresh() {
  fetch('/api/snapshot')
    .then(r => r.json())
    .then(data => {
      render(data);
      document.getElementById('updated-at').textContent = new Date().toLocaleTimeString();
    })
    .catch(() => {});
}

refresh();
setInterval(refresh, 2000);
</script>
</body>
</html>
`))
