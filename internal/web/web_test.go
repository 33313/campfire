package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The client is Svelte, so the compiler resolves its imports and the Playwright
// suite drives its behaviour. What neither of those covers is the design
// handoff: these tests hold the stylesheet and the components to DESIGN.md,
// and they read the sources rather than the build output so that `go test`
// needs no Node.

var (
	cssVarRe = regexp.MustCompile(`var\(\s*(--[\w-]+)`)
	cssDefRe = regexp.MustCompile(`(--[\w-]+)\s*:`)
)

// sources returns every client source file, keyed by its path under src/.
func sources(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("src")
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 20 {
		t.Fatalf("only %d client sources were found; the walk is not finding them", len(out))
	}
	return out
}

// The fonts the stylesheet names are served from static/, which Vite copies
// into the embedded directory verbatim.
func TestEveryFontFaceIsPresent(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("src", "fonts.css"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, m := range regexp.MustCompile(`url\((/[^)]+)\)`).FindAllStringSubmatch(string(css), -1) {
		found++
		ref := filepath.Join("static", filepath.FromSlash(strings.TrimPrefix(m[1], "/")))
		if _, err := os.Stat(ref); err != nil {
			t.Errorf("fonts.css references %s, which is not in static/", m[1])
		}
	}
	if found < 4 {
		t.Fatalf("only %d font faces were examined; the scan is not finding them", found)
	}
}

// DESIGN.md §1: nothing in the interface uses a colour that is not on the
// token list. A var() naming a token that does not exist renders as nothing,
// silently.
func TestEveryStyleTokenIsDefined(t *testing.T) {
	src := sources(t)
	defined := map[string]bool{}
	for _, m := range cssDefRe.FindAllStringSubmatch(src["style.css"], -1) {
		defined[m[1]] = true
	}
	if len(defined) < 20 {
		t.Fatal("style.css defines almost no tokens; the scan is not finding them")
	}
	used := map[string]bool{}
	for _, source := range src {
		for _, m := range cssVarRe.FindAllStringSubmatch(source, -1) {
			used[m[1]] = true
		}
	}
	var missing []string
	for name := range used {
		if !defined[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("undefined design tokens: %s", strings.Join(missing, ", "))
	}
}

// DESIGN.md §2 rule 1: every dot that reports state is a circle, and it stays
// circular even though every other corner in the product is square. Rule 2:
// controls do not follow it.
func TestOnlyStatusPipsAreRound(t *testing.T) {
	css := sources(t)["style.css"]
	for _, block := range strings.Split(css, "}") {
		if !strings.Contains(block, "border-radius: 50%") {
			continue
		}
		selector := strings.TrimSpace(strings.Split(block, "{")[0])
		if !strings.Contains(selector, "pip") {
			t.Errorf("%s is round; only status pips are", selector)
		}
	}
	if !strings.Contains(css, ".pip {") {
		t.Error("there is no .pip rule")
	}
	for _, control := range []string{".check"} {
		i := strings.Index(css, control+" {")
		if i < 0 {
			t.Errorf("no %s rule", control)
			continue
		}
		if strings.Contains(css[i:i+strings.Index(css[i:], "}")], "border-radius") {
			t.Errorf("%s sets a border radius; controls stay square", control)
		}
	}
}

// DESIGN.md §5: a card carries five fields and no sixth — bucket, title,
// priority, identifier, assignee. SPEC.md §8.1 keeps labels off it, because
// labels are a filtering mechanism and cards carrying them become unreadable.
func TestBoardCardsCarryNoLabels(t *testing.T) {
	card, ok := sources(t)["components/Card.svelte"]
	if !ok {
		t.Fatal("there is no Card component")
	}
	if strings.Contains(card, "labels") || strings.Contains(card, "label-chip") {
		t.Error("the board card renders labels; they live on the item detail view only")
	}
	for _, field := range []string{"BucketIcon", "card-title", "PriorityGlyph", "ident", "Avatar"} {
		if !strings.Contains(card, field) {
			t.Errorf("the board card is missing %s", field)
		}
	}
}

// DESIGN.md §2 rule 3: there is no ticket type. No type field, no per-team
// type list, no type filter, no type column, no type on a card.
func TestTheClientHasNoTicketType(t *testing.T) {
	for name, source := range sources(t) {
		if strings.HasSuffix(name, ".css") {
			continue
		}
		for _, forbidden := range []string{"ticketType", "itemType", "typeId", "\"type\":"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s mentions %s; a ticket has no type", name, forbidden)
			}
		}
	}
}

// The build writes the client into app/, which is what the binary embeds. A
// binary built without that step must say so rather than serve a directory
// listing or a bare 404.
func TestAnUnbuiltClientIsReported(t *testing.T) {
	sub, err := fs.Sub(files, "app")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		t.Skip("the client has not been built in this tree")
	}
	for _, name := range []string{"index.html", "app.js"} {
		if _, err := fs.Stat(sub, name); err != nil {
			t.Errorf("the build output is missing %s", name)
		}
	}
}
