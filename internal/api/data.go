package api

import (
	"net/http"
	"path/filepath"

	"github.com/33313/campfire/internal/core"
)

// Data is the instance's data directory (DEV-116).
type Data struct {
	Database  string
	Config    string
	Snapshots string
	// MovesSnapshots is true when the snapshots move with the database.
	MovesSnapshots bool
	// Fixed is true when the deployment sets the data directory, as the
	// container image does.
	Fixed bool
	// Move validates dir and requests the move. The server restarts to apply
	// it.
	Move func(dir string) error
}

func (s *Server) getData(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	d := s.Data
	return map[string]any{"directory": filepath.Dir(d.Database), "database": d.Database,
		"config": d.Config, "snapshots": d.Snapshots, "movesSnapshots": d.MovesSnapshots,
		"fixed": d.Fixed}, nil
}

func (s *Server) moveData(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := core.RequireInstanceAdmin(a); err != nil {
		return nil, err
	}
	var in struct{ Directory string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if s.Data.Fixed || s.Data.Move == nil {
		return nil, &core.Error{Code: core.CodeInvalid, Field: "directory", Message: "this deployment decides where the data lives"}
	}
	if err := s.Data.Move(in.Directory); err != nil {
		return nil, &core.Error{Code: core.CodeInvalid, Field: "directory", Message: err.Error()}
	}
	return map[string]string{"directory": filepath.Clean(in.Directory)}, nil
}
