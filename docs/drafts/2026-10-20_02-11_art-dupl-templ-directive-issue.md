# art-dupl issue draft — templ markup can't carry suppression markers without shipping them

Title: `templ: suppression directives force rendered-output markup`

## Symptom

In templ files, clone pairs inside markup (conditional spans, repeated
attribute blocks) can't carry an `art-dupl:accept` directive as a Go
comment — a `//` comment inside templ markup renders as literal page
text. The directive grammar requires the `//` marker
(internal/accept/accept.go:28, `acceptDirectiveRe = //\s*art-dupl:accept`),
so markup-level clones stay unreportable.

## What I verified before writing this

art-dupl 0.9.0-93432fd. The regex matches the marker anywhere in a line,
not only in Go-comment position — so this works today:

```templ
<span>{ label }{ fmt.Sprintf("%d", n) }</span>
<!-- // art-dupl:accept templ markup conditional-span twin -->
```

Control run without the directive line: `1 clone group` shown. With it:
`0 shown (1 filtered suppressed)`. So suppression is reachable from
templ markup; the residual problem is that the HTML comment is passed
through to the rendered output — every dashboard page ships its
suppression markers to the browser.

Per-path exclusion (the other escape hatch) already exists
(`excludePatterns` config / `--exclude-pattern`), but excluding the
whole file turns off detection for real clones too, so it doesn't solve
this either.

## Ask

templ-aware directives: also recognize the marker inside HTML comments
(e.g. `<!-- art-dupl:accept … -->` without the `//`), or a per-clone
exclusion keyed by the same group-hash mechanism the Go-comment path
uses, so the suppression lives in source without leaking into rendered
markup.

## To verify

Same minimal pair as above: directive in an HTML comment suppresses;
directive absent shows the group; rendered HTML output contains no
`art-dupl:accept` marker.

💘 Generated with Crush
