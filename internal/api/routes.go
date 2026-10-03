package api

import (
	"net/http"
	"time"

	"github.com/33313/campfire/internal/core"
	"github.com/33313/campfire/internal/store"
)

// Every route here corresponds to exactly one operation in api/openapi.yaml.
// The MCP tool list is derived from the same correspondence over the exposed
// subset (SPEC.md §14.1, §14.3).
func (s *Server) routes() {
	// Instance and authentication.
	s.handle("GET /api/instance", s.getInstance)
	s.handle("POST /api/instance/bootstrap", s.bootstrap)
	s.handle("POST /api/auth/signin", s.signIn)
	s.handle("POST /api/auth/signout", s.signOut)
	s.handle("GET /api/auth/session", s.session)
	s.handle("POST /api/auth/redeem", s.redeem)
	s.handle("POST /api/auth/password", s.changePassword)
	s.handle("POST /api/auth/totp/setup", s.setupTOTP)
	s.handle("POST /api/auth/totp/enable", s.enableTOTP)
	s.handle("POST /api/auth/totp/disable", s.disableTOTP)

	// Users and credentials.
	s.handle("GET /api/users", s.listUsers)
	s.handle("GET /api/users/{username}/profile", s.profile)
	s.handle("POST /api/users", s.createUser)
	s.handle("PATCH /api/users/{id}", s.updateUser)
	s.handle("POST /api/users/{id}/reset", s.createReset)
	s.handle("PATCH /api/profile", s.updateProfile)
	s.handle("POST /api/invitations", s.createInvitation)
	s.handle("GET /api/agent-tokens", s.listAgentTokens)
	s.handle("POST /api/agent-tokens", s.issueAgentToken)
	s.handle("DELETE /api/agent-tokens/{id}", s.revokeAgentToken)

	// Teams, membership and work states.
	s.handle("GET /api/teams", s.listTeams)
	s.handle("POST /api/teams", s.createTeam)
	s.handle("GET /api/teams/{id}", s.getTeam)
	s.handle("PATCH /api/teams/{id}", s.updateTeam)
	s.handle("POST /api/teams/{id}/archive", s.archiveTeam)
	s.handle("GET /api/teams/{id}/members", s.listMembers)
	s.handle("PUT /api/teams/{id}/members/{userId}", s.setMember)
	s.handle("DELETE /api/teams/{id}/members/{userId}", s.removeMember)
	s.handle("GET /api/teams/{id}/states", s.listStates)
	s.handle("POST /api/teams/{id}/states", s.createState)
	s.handle("PATCH /api/states/{id}", s.updateState)
	s.handle("DELETE /api/states/{id}", s.deleteState)

	// Views over items.
	s.handle("GET /api/teams/{id}/board", s.board)
	s.handle("GET /api/teams/{id}/backlog", s.backlog)
	s.handle("GET /api/teams/{id}/views", s.listViews)
	s.handle("POST /api/views", s.createView)
	s.handle("PATCH /api/views/{id}", s.updateView)
	s.handle("DELETE /api/views/{id}", s.deleteView)
	s.handle("GET /api/buckets", s.bucketIndex)

	// Items.
	s.handle("GET /api/items", s.listItems)
	s.handle("POST /api/items", s.createItem)
	s.handle("POST /api/items/bulk", s.bulkEdit)
	s.handle("GET /api/items/{id}", s.getItem)
	s.handle("PATCH /api/items/{id}", s.updateItem)
	s.handle("DELETE /api/items/{id}", s.deleteItem)
	s.handle("GET /api/items/{id}/impact", s.itemImpact)
	s.handle("POST /api/items/{id}/move", s.moveItem)
	s.handle("POST /api/items/{id}/rank", s.rankItem)
	s.handle("POST /api/items/{id}/clone", s.cloneItem)
	s.handle("POST /api/items/{id}/split", s.splitItem)
	s.handle("POST /api/items/{id}/reassign", s.reassignBucket)
	s.handle("GET /api/items/{id}/contents", s.bucketContents)
	s.handle("POST /api/items/{id}/labels", s.addLabel)
	s.handle("DELETE /api/items/{id}/labels/{name}", s.removeLabel)
	s.handle("POST /api/items/{id}/relations", s.addRelation)
	s.handle("DELETE /api/relations/{id}", s.removeRelation)
	s.handle("GET /api/items/{id}/comments", s.listComments)
	s.handle("POST /api/items/{id}/comments", s.addComment)
	s.handle("PATCH /api/comments/{id}", s.updateComment)
	s.handle("DELETE /api/comments/{id}", s.deleteComment)
	s.handle("GET /api/lookup/{identifier}", s.lookup)
	s.handle("GET /api/labels", s.listLabels)

	// Snapshots.
	s.handle("GET /api/snapshots", s.listSnapshots)
	s.handle("POST /api/snapshots", s.createSnapshot)
	s.handle("POST /api/restore", s.restoreSnapshot)

	// The audit log, and the stream that resumes from it.
	s.handle("GET /api/log", s.log)
	// Live updates are not described by the OpenAPI document, which does not
	// usefully describe a long-lived stream. Their contract is SPEC.md §12.
	s.mux.HandleFunc("GET /api/stream", s.streamHandler)
}

// Instance and authentication.

func (s *Server) getInstance(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	needs, err := s.svc.NeedsBootstrap(r.Context())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"needsBootstrap": needs,
		"bucketIcons":    store.BucketIcons,
		"bucketColors":   store.BucketColours,
		"hiddenTabGrace": s.HiddenTabGrace.Milliseconds(),
	}, nil
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Username, Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.Bootstrap(r.Context(), in.Username, in.Password)
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Username, Password, Code string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	res, err := s.svc.SignIn(r.Context(), in.Username, in.Password, in.Code)
	if err != nil {
		return nil, err
	}
	if res.Token != "" {
		s.setSession(w, r, res.Token)
		// The token travels in the cookie only; echoing it into the body would
		// put a credential somewhere a script can read it.
		res.Token = ""
	}
	return res, nil
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteLaxMode,
		Expires: time.Now().Add(30 * 24 * time.Hour),
	})
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if err := s.svc.SignOut(r.Context(), a); err != nil {
		return nil, err
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	return nil, nil
}

func (s *Server) session(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	if a.UserID == 0 {
		return nil, &core.Error{Code: core.CodeUnauthenticated, Message: "sign in first"}
	}
	teams, err := s.svc.MyTeams(r.Context(), a)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user":  map[string]any{"id": a.UserID, "username": a.Username, "isAdmin": a.IsAdmin},
		"teams": teams,
	}, nil
}

func (s *Server) redeem(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Token, Username, Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.RedeemInvitation(r.Context(), in.Token, in.Username, in.Password)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Current, Next string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if err := s.svc.ChangePassword(r.Context(), a, in.Current, in.Next); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Server) setupTOTP(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	secret, uri, err := s.svc.SetupTOTP(r.Context(), a)
	if err != nil {
		return nil, err
	}
	return map[string]string{"secret": secret, "uri": uri}, nil
}

func (s *Server) enableTOTP(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Secret, Code string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.EnableTOTP(r.Context(), a, in.Secret, in.Code)
}

func (s *Server) disableTOTP(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Password string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.DisableTOTP(r.Context(), a, in.Password)
}

// Users and credentials.

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.ListUsers(r.Context(), a, queryInt(r, "after"), int(queryInt(r, "limit")))
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct {
		Username, Password, Email string
		IsAdmin                   bool
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.CreateUser(r.Context(), a, in.Username, in.Password, in.Email, in.IsAdmin)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	var in struct{ IsAdmin bool }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.SetUserAdmin(r.Context(), a, id, in.IsAdmin)
}

func (s *Server) createReset(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return s.svc.CreatePasswordReset(r.Context(), a, id)
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Email, Avatar, Bio string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return nil, s.svc.UpdateProfile(r.Context(), a, in.Email, in.Avatar, in.Bio)
}

// A user as everyone else sees them, addressed by the name in the URL rather
// than an id: the address a profile link carries is /users/marta.
func (s *Server) profile(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.Profile(r.Context(), a, r.PathValue("username"))
}

func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	return s.svc.CreateInvitation(r.Context(), a)
}

// listAgentTokens defaults to the caller's own tokens. An explicit userId of 0
// means every token in the instance, which only an instance administrator may
// ask for (SPEC.md §14.2).
func (s *Server) listAgentTokens(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	userID := a.UserID
	if r.URL.Query().Has("userId") {
		userID = queryInt(r, "userId")
	}
	return s.svc.ListAgentTokens(r.Context(), a, userID)
}

func (s *Server) issueAgentToken(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	var in struct{ Name string }
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	return s.svc.IssueAgentToken(r.Context(), a, in.Name)
}

func (s *Server) revokeAgentToken(w http.ResponseWriter, r *http.Request, a core.Actor) (any, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	return nil, s.svc.RevokeAgentToken(r.Context(), a, id)
}
