package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Appearance is how bucket icons and colours are offered (DEV-79).
type Appearance struct {
	ColourMode    string   `json:"colourMode"`
	HiddenGroups  []string `json:"hiddenGroups"`
	HiddenColours []string `json:"hiddenColours"`
}

func GetAppearance(ctx context.Context, q Q) (Appearance, error) {
	var mode, groups, colours string
	err := q.QueryRowContext(ctx, `SELECT colour_mode, hidden_groups, hidden_colours FROM appearance`).
		Scan(&mode, &groups, &colours)
	if errors.Is(err, sql.ErrNoRows) {
		return Appearance{ColourMode: "palette", HiddenGroups: []string{}, HiddenColours: []string{}}, nil
	}
	return Appearance{ColourMode: mode, HiddenGroups: strings.Fields(groups),
		HiddenColours: strings.Fields(colours)}, err
}

func SetAppearance(ctx context.Context, q Q, a Appearance) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO appearance (id, colour_mode, hidden_groups, hidden_colours) VALUES (1, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET colour_mode = excluded.colour_mode,
			hidden_groups = excluded.hidden_groups, hidden_colours = excluded.hidden_colours`,
		a.ColourMode, strings.Join(a.HiddenGroups, " "), strings.Join(a.HiddenColours, " "))
	return err
}
