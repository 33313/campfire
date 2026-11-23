package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Theme struct {
	Key    string            `json:"key"`
	Name   string            `json:"name"`
	Colors map[string]string `json:"colors"`
}

type ThemeSettings struct {
	Selected string  `json:"selected"`
	Themes   []Theme `json:"themes"`
}

// An owner of zero addresses the instance; other settings are kept with the
// account so deleting an account removes its personal themes as well.
func GetThemeSettings(ctx context.Context, q Q, owner int64) (ThemeSettings, error) {
	s := ThemeSettings{Themes: []Theme{}}
	var raw string
	var err error
	if owner == 0 {
		err = q.QueryRowContext(ctx, `SELECT theme_settings FROM appearance`).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return s, nil
		}
	} else {
		err = q.QueryRowContext(ctx, `SELECT theme_settings FROM users WHERE id = ?`, owner).Scan(&raw)
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal([]byte(raw), &s)
	return s, err
}

func SetThemeSettings(ctx context.Context, q Q, owner int64, s ThemeSettings) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if owner == 0 {
		_, err = q.ExecContext(ctx, `INSERT INTO appearance (id, colour_mode, hidden_groups, hidden_colours, theme_settings)
			VALUES (1, 'palette', '', '', ?) ON CONFLICT(id) DO UPDATE SET theme_settings = excluded.theme_settings`, string(raw))
	} else {
		_, err = q.ExecContext(ctx, `UPDATE users SET theme_settings = ? WHERE id = ?`, string(raw), owner)
	}
	return err
}
