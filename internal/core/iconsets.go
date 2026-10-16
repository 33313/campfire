package core

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/33313/campfire/internal/store"
)

// An instance administrator may add bucket icons of the instance's own, as a
// set under a prefix they choose (DEV-79). A bucket stores prefix.name, which
// no built-in key can be, since none has a dot.
//
// An uploaded SVG is never stored as it came. Its shapes are read and written
// out again from an allowlist of elements and the attributes that place them,
// so a script, a link, a fill or a style cannot survive the trip; the client
// draws what is left in the bucket's stroke, like every other icon.

type NewIconSet struct {
	Name   string     `json:"name"`
	Prefix string     `json:"prefix"`
	Files  []IconFile `json:"files"`
}

type IconFile struct {
	Name string `json:"name"`
	SVG  string `json:"svg"`
}

// SkippedIcon is a file that did not become an icon, and why.
type SkippedIcon struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type IconSetUpload struct {
	Set     store.IconSet `json:"set"`
	Skipped []SkippedIcon `json:"skipped"`
}

const (
	maxIconFile = 8 << 10 // an icon's SVG as uploaded; 100 of them fit in a request
	maxIconBody = 8 << 10 // what is kept of it
	maxSetIcons = 100
)

var (
	iconPrefix = regexp.MustCompile(`^[a-z][a-z0-9-]{1,15}$`)
	notName    = regexp.MustCompile(`[^a-z0-9]+`)
	number     = `-?(?:\d+\.?\d*|\.\d+)`
	viewBoxRe  = regexp.MustCompile(`^\s*(` + number + `)[\s,]+(` + number + `)[\s,]+(` + number + `)[\s,]+(` + number + `)\s*$`)
	lengthRe   = regexp.MustCompile(`^\s*(` + number + `)\s*(?:px)?\s*$`)
)

func (s *Service) IconSets(ctx context.Context) ([]store.IconSet, error) {
	sets, err := store.IconSets(ctx, s.DB.R)
	return sets, mapErr(err)
}

// CreateIconSet keeps every file that is an icon and says why each other one
// is not. A set with no icon in it is refused.
func (s *Service) CreateIconSet(ctx context.Context, a Actor, in NewIconSet) (*IconSetUpload, error) {
	if err := RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 40 {
		return nil, invalidField("name", "a set's name is between 1 and 40 characters")
	}
	prefix := strings.TrimSpace(in.Prefix)
	if !iconPrefix.MatchString(prefix) {
		return nil, invalidField("prefix", "a prefix is 2 to 16 lower-case letters, digits and hyphens, starting with a letter")
	}
	if len(in.Files) == 0 {
		return nil, invalidField("files", "a set needs at least one SVG")
	}
	if len(in.Files) > maxSetIcons {
		return nil, invalidField("files", "a set has at most %d icons", maxSetIcons)
	}

	out := &IconSetUpload{Set: store.IconSet{Name: name, Prefix: prefix, Icons: []store.Icon{}}, Skipped: []SkippedIcon{}}
	seen := map[string]bool{}
	for _, f := range in.Files {
		icon := iconName(f.Name)
		skip := func(reason string) { out.Skipped = append(out.Skipped, SkippedIcon{File: f.Name, Reason: reason}) }
		switch {
		case icon == "":
			skip("its name has no letters or digits")
		case seen[icon]:
			skip("another file is already " + icon)
		case len(f.SVG) > maxIconFile:
			skip(fmt.Sprintf("it is over %dKB", maxIconFile>>10))
		default:
			viewBox, body, err := cleanSVG(f.SVG)
			if err != nil {
				skip(err.Error())
				continue
			}
			seen[icon] = true
			out.Set.Icons = append(out.Set.Icons, store.Icon{Key: prefix + "." + icon, Name: icon, ViewBox: viewBox, Body: body})
		}
	}
	if len(out.Set.Icons) == 0 {
		return nil, invalidField("files", "none of the files is an icon: %s %s", out.Skipped[0].File, out.Skipped[0].Reason)
	}

	err := s.write(ctx, a, func(t *wtx) error {
		taken, err := store.PrefixTaken(ctx, t.Tx, prefix)
		if err != nil {
			return err
		}
		if taken {
			return invalidField("prefix", "another set has the prefix %s", prefix)
		}
		out.Set.CreatedAt = t.now
		if out.Set.ID, err = store.CreateIconSet(ctx, t.Tx, out.Set, a.UserID); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "icon_set",
			NewValue: fmt.Sprintf("%s (%s.*, %d icons)", name, prefix, len(out.Set.Icons))})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetIconSetHidden hides a set from pickers, or offers it again. Buckets that
// wear its icons keep them.
func (s *Service) SetIconSetHidden(ctx context.Context, a Actor, id int64, hidden bool) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		set, err := store.IconSetByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := store.SetIconSetHidden(ctx, t.Tx, id, hidden); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "icon_set_hidden",
			OldValue: set.Prefix, NewValue: strconv.FormatBool(hidden)})
	})
}

// DeleteIconSet removes a set. Its buckets keep their keys and are drawn with
// the default icon, until a set with those keys is uploaded again.
func (s *Service) DeleteIconSet(ctx context.Context, a Actor, id int64) error {
	if err := RequireInstanceAdmin(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		set, err := store.IconSetByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		if err := store.DeleteIconSet(ctx, t.Tx, id); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvInstance, Entity: "instance", Field: "icon_set",
			OldValue: set.Name + " (" + set.Prefix + ".*)"})
	})
}

// validIcon says whether a bucket may carry key: a built-in one, or one of a
// set's, hidden or not.
func validIcon(ctx context.Context, q store.Q, key string) error {
	if store.ValidIcon(key) {
		return nil
	}
	ok, err := store.CustomIconExists(ctx, q, key)
	if err != nil {
		return err
	}
	if !ok {
		return invalidField("icon", "that is not one of the bucket icons")
	}
	return nil
}

// iconName is a file's name as an icon key's second half: "Rocket Lab.svg"
// is rocket-lab.
func iconName(file string) string {
	stem := strings.TrimSuffix(path.Base(strings.ReplaceAll(file, `\`, "/")), path.Ext(file))
	name := strings.Trim(notName.ReplaceAllString(strings.ToLower(stem), "-"), "-")
	if len(name) > 40 {
		name = strings.TrimRight(name[:40], "-")
	}
	return name
}

// The shapes an icon may be drawn with, and the attributes that place them.
var (
	iconShapes = map[string]bool{"path": true, "circle": true, "ellipse": true, "rect": true,
		"line": true, "polyline": true, "polygon": true, "g": true}
	iconAttrs = map[string]bool{"d": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true,
		"x": true, "y": true, "width": true, "height": true, "x1": true, "y1": true, "x2": true, "y2": true,
		"points": true, "transform": true}
	// Read past without complaint: they draw nothing once fills and styles are gone.
	iconQuiet = map[string]bool{"title": true, "desc": true, "metadata": true, "defs": true, "style": true}
)

const svgNS = "http://www.w3.org/2000/svg"

// cleanSVG reads an SVG and gives back its viewBox and its shapes, written out
// again with only the attributes that place them.
func cleanSVG(src string) (viewBox, body string, err error) {
	dec := xml.NewDecoder(strings.NewReader(src))
	var b strings.Builder
	depth, skip, shapes := 0, 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", "", errors.New("it is not well-formed XML")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 {
				skip++
				continue
			}
			attr := map[string]string{}
			for _, a := range t.Attr {
				if a.Name.Space == "" {
					attr[a.Name.Local] = a.Value
				}
			}
			filled := attr["fill"] != "" && attr["fill"] != "none"
			if depth == 0 {
				if filled {
					return "", "", errors.New("it has fills, and an icon is strokes only")
				}
				if t.Name.Local != "svg" {
					return "", "", errors.New("it is not an SVG")
				}
				if viewBox, err = readViewBox(attr); err != nil {
					return "", "", err
				}
				depth++
				continue
			}
			name := t.Name.Local
			// Another namespace's element (an editor's own bookkeeping), a
			// quiet one, or a shape that is not stroked draws nothing here.
			if (t.Name.Space != "" && t.Name.Space != svgNS) || iconQuiet[name] || attr["stroke"] == "none" {
				skip = 1
				continue
			}
			if filled {
				return "", "", errors.New("it has fills, and an icon is strokes only")
			}
			if !iconShapes[name] {
				return "", "", fmt.Errorf("it has a <%s>, and an icon is shapes only", name)
			}
			depth++
			if name != "g" {
				shapes++
			}
			b.WriteString("<" + name)
			for _, a := range t.Attr {
				if a.Name.Space == "" && iconAttrs[a.Name.Local] {
					b.WriteString(" " + a.Name.Local + `="` + html.EscapeString(a.Value) + `"`)
				}
			}
			b.WriteString(">")
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			depth--
			if depth > 0 {
				b.WriteString("</" + t.Name.Local + ">")
			}
		}
	}
	if viewBox == "" {
		return "", "", errors.New("it is not an SVG")
	}
	if shapes == 0 {
		return "", "", errors.New("it draws nothing")
	}
	if b.Len() > maxIconBody {
		return "", "", fmt.Errorf("its shapes are over %dKB", maxIconBody>>10)
	}
	return viewBox, b.String(), nil
}

// readViewBox takes the root's viewBox, or makes one from its width and
// height, as "minX minY width height".
func readViewBox(attr map[string]string) (string, error) {
	if m := viewBoxRe.FindStringSubmatch(attr["viewBox"]); m != nil {
		if w, _ := strconv.ParseFloat(m[3], 64); w <= 0 {
			return "", errors.New("its viewBox has no width")
		}
		if h, _ := strconv.ParseFloat(m[4], 64); h <= 0 {
			return "", errors.New("its viewBox has no height")
		}
		return strings.Join(m[1:], " "), nil
	}
	w, h := lengthRe.FindStringSubmatch(attr["width"]), lengthRe.FindStringSubmatch(attr["height"])
	if attr["viewBox"] == "" && w != nil && h != nil {
		return "0 0 " + w[1] + " " + h[1], nil
	}
	return "", errors.New("it has no viewBox")
}
