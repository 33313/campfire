package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/33313/campfire/internal/core"
)

func (s *Server) listItems(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	// A saved view is applied by naming it in view, and-ed like the rest
	// (SPEC.md §9.4).
	f := filterFrom(r)
	return s.svc.ListItems(r.Context(), a, f, r.URL.Query().Get("cursor"), int(queryInt(r, "limit")))
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in core.NewItem
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateItem(r.Context(), a, in)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.GetItem(r.Context(), a, id)
}

// lookup resolves a permalink such as SOL-42. One format means one pattern for
// finding references in text (SPEC.md §6.3).
func (s *Server) lookup(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.GetItemByIdentifier(r.Context(), a, r.PathValue("identifier"))
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in core.ItemPatch
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.UpdateItem(r.Context(), a, id, in)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.DeleteItem(r.Context(), a, id)
}

// itemImpact is what a deletion confirm shows before it happens: how many
// items across how many teams (SPEC.md §6.4).
func (s *Server) itemImpact(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	items, teams, err := s.svc.ItemImpact(r.Context(), a, id)
	if err != nil {
		return nil, err
	}
	return map[string]int{"items": items, "teams": teams}, nil
}

func (s *Server) moveItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		StateID      int64
		Reason, Note string
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.MoveItem(r.Context(), a, id, in.StateID, in.Reason, in.Note)
}

func (s *Server) rankItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ StateID, AfterID, BeforeID int64 }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.RankItem(r.Context(), a, id, in.StateID, in.AfterID, in.BeforeID)
}

func (s *Server) cloneItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.Clone(r.Context(), a, id)
}

func (s *Server) splitItem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		Titles       []string
		RejectSource bool
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.Split(r.Context(), a, id, in.Titles, in.RejectSource)
}

func (s *Server) bulkEdit(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in core.BulkEdit
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.ApplyBulkEdit(r.Context(), a, in)
}

func (s *Server) reassignBucket(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ TeamID int64 }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.ReassignBucket(r.Context(), a, id, in.TeamID)
}

func (s *Server) bucketContents(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.BucketContents(r.Context(), a, id)
}

func (s *Server) addLabel(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ Name string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.AddLabel(r.Context(), a, id, in.Name)
}

func (s *Server) removeLabel(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.RemoveLabel(r.Context(), a, id, r.PathValue("name"))
}

func (s *Server) addRelation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct {
		ToID int64
		Kind string
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.AddRelation(r.Context(), a, id, in.ToID, in.Kind)
}

func (s *Server) removeRelation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.RemoveRelation(r.Context(), a, id)
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.ListComments(r.Context(), a, id, queryInt(r, "after"), int(queryInt(r, "limit")))
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ Body string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.AddComment(r.Context(), a, id, in.Body)
}

func (s *Server) updateComment(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ Body string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.UpdateComment(r.Context(), a, id, in.Body)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.DeleteComment(r.Context(), a, id)
}

// Pictures pasted into descriptions and comments (DEV-40). The body of an
// upload is the picture itself, not JSON, and what comes back is the address
// the Markdown names it by.
func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, s.MaxImage+1))
	if err != nil {
		return nil, &core.Error{Code: core.CodeInvalid, Message: "the picture did not arrive whole"}
	}
	id, err := s.svc.UploadImage(r.Context(), a, data, s.MaxImage)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "url": "/api/images/" + strconv.FormatInt(id, 10)}, nil
}

func (s *Server) getImage(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	img, err := s.svc.Image(r.Context(), a, id)
	if err != nil {
		return nil, err
	}
	return file{img.ContentType, img.Data}, nil
}
