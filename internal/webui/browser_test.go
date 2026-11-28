//go:build unix

package webui

// Browser render-budget gate (WebUI/UX review follow-ups 2026-11-28,
// item 1): the server can bound rows, but only a real layout engine can
// notice the 26,927-pixel page or a horizontal overflow at 390px. This
// file drives the seeded fixture journal through headless chromium and
// asserts the layout budget at 390/768/1440. It skips cleanly when no
// chromium-family browser is present (nix checkPhase, GH runners) — the
// gates that own browsers wire CHROME_BIN explicitly.

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// layoutBudgets pins the document height ceiling per viewport. Measured
// 2026-11-28 on the seeded fixture journal (537 dead / 220 pending,
// chromium 158 headless): dashboard 4283/4204/3645 and board
// 3197/3158/3335 px at 390/768/1440 — the ceiling is 2× the measured
// height, a regression wall against the 26,927-px incident page, not a
// pixel-perfect snapshot.
type layoutBudget struct {
	width, height int
	maxDashboard  float64
	maxBoard      float64
}

var layoutBudgets = []layoutBudget{
	{width: 390, height: 844, maxDashboard: 8600, maxBoard: 6400},
	{width: 768, height: 1024, maxDashboard: 8500, maxBoard: 6400},
	{width: 1440, height: 1200, maxDashboard: 7300, maxBoard: 6700},
}

// chromeBinary resolves the browser for the browser-gated gates: explicit
// env wins (TQ_CHROME_BIN, then CHROME_BIN), then PATH. Skip — never
// fail — when absent, so hermetic environments stay green.
func chromeBinary(t *testing.T) string {
	t.Helper()

	for _, env := range []string{"TQ_CHROME_BIN", "CHROME_BIN"} {
		if p := os.Getenv(env); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}

			t.Fatalf("%s=%s does not exist", env, p)
		}
	}

	for _, c := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome-headless-shell"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}

	t.Skip("no chromium-family browser (set CHROME_BIN, e.g. nix build nixpkgs#chromium); browser gates skipped")

	return ""
}

func browserCtx(t *testing.T, bin string, w, h int) (context.Context, context.CancelFunc, chromedp.Action[chromedp.Void]) {
	t.Helper()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(bin),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserDataDir(t.TempDir()),
	)

	parent, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	ctx, cancelTab := chromedp.NewContext(allocCtx)

	// Chromium clamps OS windows to a 500px minimum width, so the viewport
	// is emulated via CDP instead of --window-size: a 390px OS window is
	// a 500px page, and the "no overflow at 390" claim would measure the
	// wrong width.
	return ctx, func() {
		cancelTab()
		cancelAlloc()
		cancel()
	}, chromedp.EmulateViewport(int64(w), int64(h))
}

// pageMetrics is the layout budget probe result.
type pageMetrics struct {
	ScrollHeight float64 `json:"h"`
	ScrollWidth  float64 `json:"w"`
	InnerWidth   float64 `json:"vw"`
}

const metricsJS = `(() => { const d = document.documentElement;
return {h: d.scrollHeight, w: d.scrollWidth, vw: window.innerWidth}; })()`

const rowCountsJS = `(() => {
const q = (sel) => document.querySelectorAll(sel).length;
return {
dead: q('#frag-dlq tbody tr'),
settled: q('#frag-table details.tq-settled tbody tr'),
active: q('#frag-table > div.tq-panel tbody tr'),
feed: q('#frag-feed [data-fact-type]'),
}; })()`

type domRowCounts struct {
	Dead    int `json:"dead"`
	Settled int `json:"settled"`
	Active  int `json:"active"`
	Feed    int `json:"feed"`
}

// TestRenderBudgetLayout drives the seeded fixture journal through real
// chromium: no horizontal overflow at 390/768/1440, document heights
// under the pinned ceilings, and the DOM row counts matching the
// server-side bounds (renderbudget_test.go).
func TestRenderBudgetLayout(t *testing.T) {
	bin := chromeBinary(t)

	ts, deadID := seededFixtureServer(t)

	for _, b := range layoutBudgets {
		t.Run(strconv.Itoa(b.width)+"x"+strconv.Itoa(b.height), func(t *testing.T) {
			ctx, cancel, viewport := browserCtx(t, bin, b.width, b.height)
			defer cancel()

			for _, path := range []string{"/", "/?view=board", "/task/" + deadID} {
				if _, err := chromedp.Run(ctx, chromedp.Steps(chromedp.Navigate(ts.URL+path), viewport)); err != nil {
					t.Fatalf("navigate %s: %v", path, err)
				}

				m, err := chromedp.Run(ctx, chromedp.Evaluate[pageMetrics](metricsJS))
				if err != nil {
					t.Fatalf("evaluate %s: %v", path, err)
				}

				if m.ScrollWidth > m.InnerWidth {
					t.Errorf(
						"%s at %dpx: document overflows horizontally (scrollWidth %.0f > innerWidth %.0f)",
						path,
						b.width,
						m.ScrollWidth,
						m.InnerWidth,
					)
				}

				if path == "/" && m.ScrollHeight > b.maxDashboard {
					t.Errorf(
						"%s at %dpx: dashboard height %.0f exceeds pinned ceiling %.0f",
						path,
						b.width,
						m.ScrollHeight,
						b.maxDashboard,
					)
				}

				if path == "/?view=board" && m.ScrollHeight > b.maxBoard {
					t.Errorf(
						"%s at %dpx: board height %.0f exceeds pinned ceiling %.0f",
						path,
						b.width,
						m.ScrollHeight,
						b.maxBoard,
					)
				}

				t.Logf("%s at %dpx: h=%.0f w=%.0f vw=%.0f", path, b.width, m.ScrollHeight, m.ScrollWidth, m.InnerWidth)
			}
		})
	}

	// Row counts are width-independent; assert them once at 1440.
	ctx, cancel, viewport := browserCtx(t, bin, 1440, 1200)
	defer cancel()

	if _, err := chromedp.Run(ctx, chromedp.Steps(chromedp.Navigate(ts.URL+"/"), viewport)); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	rows, err := chromedp.Run(ctx, chromedp.Evaluate[domRowCounts](rowCountsJS))
	if err != nil {
		t.Fatalf("evaluate row counts: %v", err)
	}

	if rows.Dead > deadTableLimit {
		t.Errorf("browser rendered %d DLQ rows, bound is deadTableLimit=%d", rows.Dead, deadTableLimit)
	}

	if rows.Settled > taskTableLimit {
		t.Errorf("browser rendered %d settled rows, bound is taskTableLimit=%d", rows.Settled, taskTableLimit)
	}

	if rows.Active > activeStripLimit {
		t.Errorf("browser rendered %d active-strip rows, bound is activeStripLimit=%d", rows.Active, activeStripLimit)
	}

	if rows.Feed != factFeedLen {
		t.Errorf("browser rendered %d feed lines, want exactly factFeedLen=%d", rows.Feed, factFeedLen)
	}

	t.Logf("dom rows: dead=%d settled=%d active=%d feed=%d", rows.Dead, rows.Settled, rows.Active, rows.Feed)
}
