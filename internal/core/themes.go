package core

import (
	"context"
	"database/sql"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/33313/campfire/internal/store"
)

var themeKey = regexp.MustCompile(`^[0-9a-f]{16}$`)
var themeColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var themeTokens = strings.Fields("page sunk surface bar border border-strong rule-faint text bright muted muted-soft dim faint ghost accent accent-hover alert ok pr-merged on-accent")
var builtinThemes = []string{"campfire", "dracula", "tokyo-night"}

type Themes struct {
	Instance store.ThemeSettings `json:"instance"`
	Personal store.ThemeSettings `json:"personal"`
}

func (s *Service) Themes(ctx context.Context, a Actor) (out Themes, err error) {
	if err = requireUser(a); err != nil {
		return out, err
	}
	out.Instance, err = store.GetThemeSettings(ctx, s.DB.R, 0)
	if err == nil {
		out.Personal, err = store.GetThemeSettings(ctx, s.DB.R, a.UserID)
	}
	return out, err
}

// InstanceTheme is read before sign-in; the rest of the library needs a session.
func (s *Service) InstanceTheme(ctx context.Context) (store.ThemeSettings, error) {
	v, err := store.GetThemeSettings(ctx, s.DB.R, 0)
	v.Themes = slices.DeleteFunc(v.Themes, func(t store.Theme) bool { return "instance:"+t.Key != v.Selected })
	return v, err
}

func (s *Service) SetThemeSettings(ctx context.Context, a Actor, scope string, in store.ThemeSettings) error {
	if err := requireUser(a); err != nil {
		return err
	}
	owner := a.UserID
	switch scope {
	case "instance":
		if err := RequireInstanceAdmin(a); err != nil {
			return err
		}
		owner = 0
	case "personal":
	default:
		return errf(CodeInvalid, "unknown theme scope")
	}
	if len(in.Themes) > 24 {
		return errf(CodeInvalid, "a theme library can hold at most 24 themes")
	}
	if in.Themes == nil {
		in.Themes = []store.Theme{}
	}
	keys := map[string]bool{}
	for i := range in.Themes {
		t := &in.Themes[i]
		t.Name = strings.TrimSpace(t.Name)
		if !themeKey.MatchString(t.Key) || keys[t.Key] || t.Name == "" || utf8.RuneCountInString(t.Name) > 60 {
			return errf(CodeInvalid, "theme keys must be unique and names must be 1 to 60 characters")
		}
		keys[t.Key] = true
		for token, color := range t.Colors {
			if !slices.Contains(themeTokens, token) || !themeColor.MatchString(color) {
				return errf(CodeInvalid, "theme colours must be hex values for known tokens")
			}
		}
	}
	return s.DB.Write(ctx, func(tx *sql.Tx) error {
		allowed := in.Selected == "" || slices.Contains(builtinThemes, in.Selected)
		if key, ok := strings.CutPrefix(in.Selected, scope+":"); ok {
			allowed = keys[key]
		} else if key, ok := strings.CutPrefix(in.Selected, "instance:"); ok && owner != 0 {
			parent, err := store.GetThemeSettings(ctx, tx, 0)
			if err != nil {
				return err
			}
			allowed = slices.ContainsFunc(parent.Themes, func(t store.Theme) bool { return t.Key == key })
		}
		if !allowed {
			return errf(CodeInvalid, "that theme is not available")
		}
		return store.SetThemeSettings(ctx, tx, owner, in)
	})
}
