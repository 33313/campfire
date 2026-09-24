package api

import (
	"net/http"

	"github.com/campfire/campfire/internal/core"
	"github.com/campfire/campfire/internal/store"
)

func (s *Server) listTeams(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.ListTeams(r.Context(), a, queryBool(r, "includeArchived"))
}

func (s *Server) createTeam(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Name, ShortCode string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateTeam(r.Context(), a, in.Name, in.ShortCode)
}

func (s *Server) getTeam(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.Team(r.Context(), a, id)
}

// updateTeam changes the name, the icon rendering, the picture and the
// description. The short code is not here: it is set at creation and MUST NOT
// change (SPEC.md §5.3).
func (s *Server) updateTeam(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		Name          *string
		IconRendering *string
		Avatar        *string
		Description   *string
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Name != nil {
		if err := s.svc.RenameTeam(r.Context(), a, id, *in.Name); err != nil {
			return nil, err
		}
	}
	if in.IconRendering != nil {
		if err := s.svc.SetIconRendering(r.Context(), a, id, *in.IconRendering); err != nil {
			return nil, err
		}
	}
	// The picture and the description are written together; one left out of
	// the body keeps what the team has.
	if in.Avatar != nil || in.Description != nil {
		t, err := s.svc.Team(r.Context(), a, id)
		if err != nil {
			return nil, err
		}
		if in.Avatar != nil {
			t.Avatar = *in.Avatar
		}
		if in.Description != nil {
			t.Description = *in.Description
		}
		if err := s.svc.SetTeamProfile(r.Context(), a, id, t.Avatar, t.Description); err != nil {
			return nil, err
		}
	}
	return s.svc.Team(r.Context(), a, id)
}

func (s *Server) archiveTeam(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ Archived bool }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.ArchiveTeam(r.Context(), a, id, in.Archived)
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.ListMembers(r.Context(), a, id)
}

func (s *Server) setMember(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	teamID, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	userID, err := pathID(r, "userId")
	if err != nil {
		return nil, err
	}
	var in struct{ Role string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.SetMember(r.Context(), a, teamID, userID, in.Role)
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	teamID, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	userID, err := pathID(r, "userId")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.RemoveMember(r.Context(), a, teamID, userID)
}

func (s *Server) listStates(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.ListWorkStates(r.Context(), a, id)
}

func (s *Server) createState(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ Name, Role string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateWorkState(r.Context(), a, id, in.Name, in.Role)
}

func (s *Server) updateState(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		Name, Role string
		Position   int64
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.UpdateWorkState(r.Context(), a, id, in.Name, in.Role, in.Position)
}

func (s *Server) deleteState(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.DeleteWorkState(r.Context(), a, id)
}

// filterFrom reads the structured filter of SPEC.md §9.2 out of the query
// string. There is no query language: these are the dimensions, each its own
// repeatable parameter.
func filterFrom(r *http.Request) core.Filter {
	return core.Filter{
		StateIDs:    queryInts(r, "state"),
		LabelIDs:    queryInts(r, "label"),
		AssigneeIDs: queryInts(r, "assignee"),
		Unassigned:  queryBool(r, "unassigned"),
		ReporterIDs: queryInts(r, "reporter"),
		BucketIDs:   queryInts(r, "bucket"),
		Priorities:  queryInts(r, "priority"),
		TeamIDs:     queryInts(r, "team"),
		Kinds:       queryStrings(r, "kind"),
		Text:        r.URL.Query().Get("q"),
	}
}

func (s *Server) board(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.Board(r.Context(), a, id, filterFrom(r),
		queryBool(r, "buckets"), int(queryInt(r, "limit")))
}

func (s *Server) backlog(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.Backlog(r.Context(), a, id, filterFrom(r),
		queryBool(r, "buckets"), int(queryInt(r, "limit")))
}

func (s *Server) listViews(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.ListSavedViews(r.Context(), a, id)
}

func (s *Server) createView(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in core.ViewInput
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateSavedView(r.Context(), a, in)
}

func (s *Server) updateView(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in core.ViewInput
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.UpdateSavedView(r.Context(), a, id, in)
}

func (s *Server) deleteView(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.DeleteSavedView(r.Context(), a, id)
}

func (s *Server) bucketIndex(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = core.ScopeParticipation
	}
	cards, cursor, err := s.svc.BucketIndex(r.Context(), a, queryInt(r, "team"), scope,
		r.URL.Query().Get("cursor"), int(queryInt(r, "limit")))
	if err != nil {
		return nil, err
	}
	return map[string]any{"buckets": cards, "cursor": cursor}, nil
}

func (s *Server) listLabels(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.ListLabels(r.Context(), a)
}

func (s *Server) log(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	f := store.EventFilter{
		ItemID:    queryInt(r, "item"),
		TeamID:    queryInt(r, "team"),
		ActorID:   queryInt(r, "actor"),
		Field:     r.URL.Query().Get("field"),
		AgentOnly: queryBool(r, "agent"),
	}
	return s.svc.Log(r.Context(), a, f, queryInt(r, "before"), int(queryInt(r, "limit")))
}
