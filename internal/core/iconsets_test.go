package core

import (
	"strings"
	"testing"

	"github.com/33313/campfire/internal/store"
)

// An uploaded SVG comes back as its shapes and nothing else (DEV-79).
func TestCleanSVGKeepsOnlyShapes(t *testing.T) {
	for _, c := range []struct{ name, src, viewBox, body, err string }{
		{name: "lucide",
			src:     `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 4h16"/><circle cx="12" cy="12" r="3"/></svg>`,
			viewBox: "0 0 24 24", body: `<path d="M4 4h16"></path><circle cx="12" cy="12" r="3"></circle>`},
		{name: "tabler's box that is not stroked",
			src:     `<svg viewBox="0 0 24 24" fill="none"><path stroke="none" d="M0 0h24v24H0z" fill="none"/><path d="M5 12h14"/></svg>`,
			viewBox: "0 0 24 24", body: `<path d="M5 12h14"></path>`},
		{name: "an editor's leftovers",
			src:     `<?xml version="1.0"?><!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg" xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd" width="16px" height="16"><title>x</title><defs><linearGradient fill="red"/></defs><sodipodi:namedview/><g transform="rotate(45 8 8)" class="a" style="fill:red"><line x1="2" y1="8" x2="14" y2="8" onclick="alert(1)"/></g></svg>`,
			viewBox: "0 0 16 16", body: `<g transform="rotate(45 8 8)"><line x1="2" y1="8" x2="14" y2="8"></line></g>`},
		{name: "a value that tries to leave its quotes",
			src:     `<svg viewBox="0 0 16 16"><path d="M0 0&quot; onload=&quot;x"/></svg>`,
			viewBox: "0 0 16 16", body: `<path d="M0 0&#34; onload=&#34;x"></path>`},
		{name: "filled", src: `<svg viewBox="0 0 16 16"><path fill="currentColor" d="M0 0h4"/></svg>`, err: "fills"},
		{name: "filled root", src: `<svg viewBox="0 0 16 16" fill="#000"><path d="M0 0h4"/></svg>`, err: "fills"},
		{name: "script", src: `<svg viewBox="0 0 16 16"><script>alert(1)</script><path d="M0 0h4"/></svg>`, err: "<script>"},
		{name: "a link", src: `<svg viewBox="0 0 16 16"><a href="https://x"><path d="M0 0h4"/></a></svg>`, err: "<a>"},
		{name: "no viewBox", src: `<svg><path d="M0 0h4"/></svg>`, err: "no viewBox"},
		{name: "empty", src: `<svg viewBox="0 0 16 16"><title>x</title></svg>`, err: "draws nothing"},
		{name: "not svg", src: `<html><path d="M0 0h4"/></html>`, err: "not an SVG"},
		{name: "broken", src: `<svg viewBox="0 0 16 16"><path d="M0 0h4"></svg>`, err: "well-formed"},
	} {
		viewBox, body, err := cleanSVG(c.src)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: err %v, want one about %q", c.name, err, c.err)
			}
			continue
		}
		if err != nil || viewBox != c.viewBox || body != c.body {
			t.Errorf("%s:\n got %q %q %v\nwant %q %q", c.name, viewBox, body, err, c.viewBox, c.body)
		}
	}
}

func TestAnIconSetIsTheAdministratorsAndOutlivesNothing(t *testing.T) {
	w := newWorld(t)
	b := w.bucket(w.sol, w.marta, "On-call")
	svg := `<svg viewBox="0 0 16 16"><path d="M2 2h12"/></svg>`
	in := NewIconSet{Name: "Ops", Prefix: "ops", Files: []IconFile{
		{Name: "Pager.svg", SVG: svg},
		{Name: "logo.svg", SVG: `<svg viewBox="0 0 16 16"><path fill="red" d="M0 0h4"/></svg>`},
		{Name: "pager.svg", SVG: svg},
	}}

	_, err := w.CreateIconSet(w.ctx, w.marta, in)
	requireCode(t, err, CodeForbidden)
	up, err := w.CreateIconSet(w.ctx, w.admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(up.Set.Icons) != 1 || up.Set.Icons[0].Key != "ops.pager" || len(up.Skipped) != 2 {
		t.Fatalf("kept %+v, skipped %+v", up.Set.Icons, up.Skipped)
	}
	_, err = w.CreateIconSet(w.ctx, w.admin, in)
	requireCode(t, err, CodeInvalid) // the prefix is taken
	bad := in
	bad.Prefix = "Ops!"
	_, err = w.CreateIconSet(w.ctx, w.admin, bad)
	requireCode(t, err, CodeInvalid)

	key, missing := "ops.pager", "ops.siren"
	if _, err := w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Icon: &key}); err != nil {
		t.Fatalf("a set's icon was refused: %v", err)
	}
	_, err = w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Icon: &missing})
	requireCode(t, err, CodeInvalid)

	// Hidden, it is still valid; deleted, the bucket keeps its key.
	if err := w.SetIconSetHidden(w.ctx, w.admin, up.Set.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Icon: &key}); err != nil {
		t.Fatalf("a hidden set's icon was refused: %v", err)
	}
	if err := w.DeleteIconSet(w.ctx, w.admin, up.Set.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.ItemByID(w.ctx, w.DB.R, b.ID)
	if err != nil || got.Icon != "ops.pager" {
		t.Fatalf("after the set went the bucket carries %q (%v)", got.Icon, err)
	}
	if sets, _ := w.IconSets(w.ctx); len(sets) != 0 {
		t.Fatalf("%d sets left", len(sets))
	}
}

func TestIconNameFromAFile(t *testing.T) {
	for in, want := range map[string]string{
		"Rocket Lab.svg": "rocket-lab", `C:\icons\a_b.SVG`: "a-b", "dir/x.svg": "x", "!!.svg": "",
	} {
		if got := iconName(in); got != want {
			t.Errorf("iconName(%q) = %q, want %q", in, got, want)
		}
	}
}
