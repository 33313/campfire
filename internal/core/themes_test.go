package core

import (
	"testing"

	"github.com/33313/campfire/internal/store"
)

func TestThemeSettingsAreScoped(t *testing.T) {
	w := newWorld(t)
	shared := store.Theme{Key: "1111111111111111", Name: "Shared", Colors: map[string]string{"accent": "#f7bf6e"}}
	private := store.Theme{Key: "2222222222222222", Name: "Private"}
	instance := store.ThemeSettings{Selected: "instance:" + shared.Key, Themes: []store.Theme{shared}}
	requireCode(t, w.SetThemeSettings(w.ctx, w.marta, "instance", instance), CodeForbidden)
	if err := w.SetThemeSettings(w.ctx, w.admin, "instance", instance); err != nil {
		t.Fatal(err)
	}
	if err := w.SetThemeSettings(w.ctx, w.marta, "personal", store.ThemeSettings{
		Selected: instance.Selected, Themes: []store.Theme{private},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := w.Themes(w.ctx, w.marta)
	if err != nil || got.Personal.Selected != instance.Selected || len(got.Personal.Themes) != 1 {
		t.Fatalf("reader's selection and personal library: %+v, %v", got, err)
	}
	other, err := w.Themes(w.ctx, w.admin)
	if err != nil || len(other.Personal.Themes) != 0 || other.Instance.Selected != instance.Selected {
		t.Fatalf("admin's separate library and shared default: %+v, %v", other, err)
	}
	requireCode(t, w.SetThemeSettings(w.ctx, w.admin, "personal", store.ThemeSettings{
		Selected: "personal:" + private.Key,
	}), CodeInvalid)
	shared.Colors["accent"] = "url(https://example.com)"
	requireCode(t, w.SetThemeSettings(w.ctx, w.admin, "instance", instance), CodeInvalid)
}
