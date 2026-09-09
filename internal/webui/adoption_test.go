package webui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The templ-components adoption table in AGENTS.md is the documentation of
// WHICH library parts this UI is allowed to lean on. Both directions rot:
// a component gets dropped from the templates but the table still claims
// it, or a new component sneaks in undocumented. This guard fails on
// either drift (round-5 M13/F67; it caught ThemeScript on its first run).

const adoptionHeading = "### templ-components adoption"

var (
	componentCallRe = regexp.MustCompile(`(display|layout|feedback)\.([A-Z][A-Za-z0-9]*)\s*\(`)
	iconRefRe       = regexp.MustCompile(`icons\.([A-Z][A-Za-z0-9]*)`)
)

func templSources(t *testing.T) string {
	t.Helper()

	files, err := filepath.Glob("*.templ")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob *.templ: %v (files: %v) — run from the webui package dir", err, files)
	}

	var sb strings.Builder

	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}

		sb.Write(body)
		sb.WriteByte('\n')
	}

	return sb.String()
}

// adoptedIdentifiers parses the AGENTS.md adoption table and returns every
// component identifier claimed as adopted. `display.Grid/StatCard` shorthand
// expands to both names.
func adoptedIdentifiers(t *testing.T) map[string]bool {
	t.Helper()

	agents, err := os.ReadFile(filepath.Join("..", "..", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}

	lines := strings.Split(string(agents), "\n")

	inSection := false

	ids := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			inSection = strings.TrimSpace(line) == adoptionHeading

			continue
		}

		if !inSection || !strings.HasPrefix(line, "|") {
			continue
		}

		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 || strings.TrimSpace(cells[1]) != "adopted" {
			continue
		}

		for raw := range strings.SplitSeq(cells[0], ",") {
			token := strings.Trim(strings.TrimSpace(raw), "` ")
			if token == "" {
				continue
			}

			parts := strings.Split(token, "/")

			prefix := ""

			if dot := strings.Index(parts[0], "."); dot >= 0 {
				prefix = parts[0][:dot+1]
			}

			for i, part := range parts {
				if i > 0 {
					part = prefix + part
				}

				if name := part; strings.Contains(name, ".") {
					ids[name[strings.Index(name, ".")+1:]] = true
				} else {
					ids[name] = true
				}
			}
		}
	}

	if len(ids) == 0 {
		t.Fatal("adoption table parsed to zero identifiers — the table moved or changed shape")
	}

	return ids
}

func TestAdoptionTableCoversTemplates(t *testing.T) {
	t.Parallel()

	src := templSources(t)
	adopted := adoptedIdentifiers(t)

	// Reverse: every component invoked from a template must be documented.
	invoked := map[string]string{}

	for _, m := range componentCallRe.FindAllStringSubmatch(src, -1) {
		invoked[m[2]] = m[0]
	}

	for _, m := range iconRefRe.FindAllStringSubmatch(src, -1) {
		invoked[m[1]] = m[0]
	}

	for name, site := range invoked {
		if !adopted[name] {
			t.Errorf(
				"template invokes %s but the AGENTS.md adoption table does not list it — add it or drop the usage",
				site,
			)
		}
	}

	// Forward: every adopted identifier must still appear in the templates.
	for name := range adopted {
		if !strings.Contains(src, name) {
			t.Errorf("AGENTS.md adoption table claims %q but no .templ source uses it — the table rotted", name)
		}
	}
}

// TestAdoptionTablePinsCustomRows: the custom (non-library) row is part of
// the same contract — the nowband, board columns/cards, and the other
// hand-rolled surfaces are documented so nobody "migrates" them to library
// components without touching the table (round-10 T26/M118).
func TestAdoptionTablePinsCustomRows(t *testing.T) {
	t.Parallel()

	agents, err := os.ReadFile(filepath.Join("..", "..", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}

	inSection := false

	customRows := 0

	for line := range strings.SplitSeq(string(agents), "\n") {
		if strings.HasPrefix(line, "### ") {
			inSection = strings.TrimSpace(line) == adoptionHeading

			continue
		}

		if !inSection || !strings.HasPrefix(line, "|") {
			continue
		}

		if strings.Contains(line, "| custom") {
			customRows++

			for _, pin := range []string{"nowband", "board"} {
				if !strings.Contains(line, pin) {
					t.Errorf("custom row lost its %q pin: %s", pin, line)
				}
			}
		}
	}

	if customRows == 0 {
		t.Fatal("adoption table lost its custom row entirely — hand-rolled surfaces are undocumented")
	}
}
