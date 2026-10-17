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

// Every palette colour clears the floor a hex colour is held to (DESIGN.md §4).
func TestThePaletteReadsOnACard(t *testing.T) {
	for _, hex := range []string{
		"#d9a05b", "#c8b45e", "#a3a961", "#7fa86a", "#8aa8a0", "#5fa7a3", "#6fa3c7", "#7d94a8",
		"#8a8fc9", "#9d82c2", "#b087a3", "#c48098", "#a27459", "#b7a587", "#8f8578", "#737d87",
	} {
		if c := contrast(hex, surface); c < minContrast {
			t.Errorf("%s stands %.2f:1 off the card", hex, c)
		}
	}
}
