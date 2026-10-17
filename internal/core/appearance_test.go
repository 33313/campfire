package core

import (
	"slices"
	"testing"
)

// DEV-79: hiding is the pickers' business. A hidden colour stays valid, and a
// hex colour is taken only while the instance says so, and only if it reads.
func TestAppearanceHidesFromPickersOnly(t *testing.T) {
	w := newWorld(t)
	b := w.bucket(w.sol, w.marta, "Release 1.0")

	off, hidden := "off", []string{"slate", "rose", "rose"}
	_, err := w.SetAppearance(w.ctx, w.marta, AppearancePatch{ColourMode: &off})
	requireCode(t, err, CodeForbidden)
	ap, err := w.SetAppearance(w.ctx, w.admin, AppearancePatch{ColourMode: &off, HiddenColours: &hidden})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ap.HiddenColours, []string{"rose", "slate"}) {
		t.Fatalf("hidden colours %v, want the palette's order without repeats", ap.HiddenColours)
	}
	bad := []string{"crimson"}
	_, err = w.SetAppearance(w.ctx, w.admin, AppearancePatch{HiddenColours: &bad})
	requireCode(t, err, CodeInvalid)

	rose := "rose"
	if _, err := w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Color: &rose}); err != nil {
		t.Fatalf("a hidden colour was refused: %v", err)
	}

	pink := "#FF88CC"
	_, err = w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Color: &pink})
	requireCode(t, err, CodeInvalid)

	hex := "hex"
	if _, err := w.SetAppearance(w.ctx, w.admin, AppearancePatch{ColourMode: &hex}); err != nil {
		t.Fatal(err)
	}
	got, err := w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Color: &pink})
	if err != nil {
		t.Fatalf("a readable hex colour was refused: %v", err)
	}
	if got.Color != "#ff88cc" {
		t.Fatalf("stored %q, want it lower-cased", got.Color)
	}
	dark := "#2a2420"
	_, err = w.UpdateItem(w.ctx, w.marta, b.ID, ItemPatch{Color: &dark})
	requireCode(t, err, CodeInvalid)
}
