package core

import (
	"context"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/33313/campfire/internal/store"
)

// How bucket icons and colours are offered is an instance administrator's
// choice (DEV-79). It hides from pickers only: a key a bucket already carries
// stays valid, so nothing stored is ever orphaned by it.

// AppearancePatch leaves a nil field alone.
type AppearancePatch struct {
	ColourMode    *string   `json:"colourMode,omitempty"`
	HiddenGroups  *[]string `json:"hiddenGroups,omitempty"`
	HiddenColours *[]string `json:"hiddenColours,omitempty"`
}

// Appearance is anyone's to read, signed in or not: the pickers need it, and
// it says nothing about what the instance holds.
func (s *Service) Appearance(ctx context.Context) (store.Appearance, error) {
	ap, err := store.GetAppearance(ctx, s.DB.R)
	return ap, mapErr(err)
}

func (s *Service) SetAppearance(ctx context.Context, a Actor, p AppearancePatch) (store.Appearance, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return store.Appearance{}, err
	}
	var out store.Appearance
	err := s.write(ctx, a, func(t *wtx) error {
		ap, err := store.GetAppearance(ctx, t.Tx)
		if err != nil {
			return err
		}
		if p.ColourMode != nil {
			if !slices.Contains([]string{"palette", "hex", "off"}, *p.ColourMode) {
				return invalidField("colourMode", "the colour mode is palette, hex or off")
			}
			ap.ColourMode = *p.ColourMode
		}
		if p.HiddenGroups != nil {
			if ap.HiddenGroups, err = subset("hiddenGroups", *p.HiddenGroups, store.BucketIconGroups); err != nil {
				return err
			}
		}
		if p.HiddenColours != nil {
			if ap.HiddenColours, err = subset("hiddenColours", *p.HiddenColours, store.BucketColours); err != nil {
				return err
			}
		}
		if err := store.SetAppearance(ctx, t.Tx, ap); err != nil {
			return err
		}
		out = ap
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "appearance",
			NewValue: "colours " + ap.ColourMode + "; hidden: " +
				strings.Join(append(slices.Clone(ap.HiddenGroups), ap.HiddenColours...), " ")})
	})
	return out, err
}

// subset keeps the keys of set that are in keys, in set's order, and refuses
// any key set does not have.
func subset(field string, keys, set []string) ([]string, error) {
	out := []string{}
	for _, k := range keys {
		if !slices.Contains(set, k) {
			return nil, invalidField(field, "%q is not one of them", k)
		}
	}
	for _, k := range set {
		if slices.Contains(keys, k) {
			out = append(out, k)
		}
	}
	return out, nil
}

var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// surface is the card a bucket's colour is read on (DESIGN.md §2).
const surface = "#1a1715"

// minContrast is the least a colour may stand off the card: WCAG's floor for
// large text and for the parts of a control, which a 9.5px chip is held to.
const minContrast = 3.0

// validColour says whether a bucket may be given c, and gives it back as it
// is stored. A palette key is always valid; #rrggbb only while the instance
// takes it, and only if it reads on a card.
func validColour(ctx context.Context, q store.Q, c string) (string, error) {
	if c == "" || store.ValidColour(c) {
		return c, nil
	}
	if !hexColour.MatchString(c) {
		return "", invalidField("color", "that is not one of the bucket colours")
	}
	ap, err := store.GetAppearance(ctx, q)
	if err != nil {
		return "", err
	}
	if ap.ColourMode != "hex" {
		return "", invalidField("color", "this instance takes only the bucket colours")
	}
	c = strings.ToLower(c)
	if contrast(c, surface) < minContrast {
		return "", invalidField("color", "%s is too dark to read on a card", c)
	}
	return c, nil
}

// contrast is WCAG 2's contrast ratio between two #rrggbb colours.
func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

func luminance(hex string) float64 {
	var l float64
	for i, w := range []float64{0.2126, 0.7152, 0.0722} {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		c := float64(v) / 255
		if c <= 0.04045 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		l += w * c
	}
	return l
}
