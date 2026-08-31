package core

import (
	"context"
	"strings"

	"github.com/campfire/campfire/internal/store"
)

// Comments are markdown, flat, and may be written by any authenticated user on
// any item, regardless of team membership (SPEC.md §5.2).
//
// Editing and deleting are the author's, plus an instance administrator's for
// deletion: the specification grants everyone the right to comment, not the
// right to rewrite someone else's words.

const maxComment = 20000

func (s *Service) AddComment(ctx context.Context, a Actor, itemID int64, body string) (*store.Comment, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, invalidField("body", "a comment needs some text")
	}
	if len(body) > maxComment {
		return nil, invalidField("body", "that comment is too long")
	}
	var out *store.Comment
	err := s.write(ctx, a, func(t *wtx) error {
		it, err := store.ItemByID(ctx, t.Tx, itemID)
		if err != nil {
			return err
		}
		id, err := store.CreateComment(ctx, t.Tx, store.Comment{
			ItemID: itemID, AuthorID: a.UserID, Body: body,
			CreatedAt: t.now, UpdatedAt: t.now})
		if err != nil {
			return err
		}
		c, err := store.CommentByID(ctx, t.Tx, id)
		if err != nil {
			return err
		}
		out = &c
		return t.log(ctx, store.Event{Kind: store.EvCommentCreated, Entity: "comment",
			EntityID: id, ItemID: itemID, TeamID: it.TeamID})
	})
	return out, err
}

func (s *Service) UpdateComment(ctx context.Context, a Actor, commentID int64, body string) (*store.Comment, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, invalidField("body", "a comment needs some text")
	}
	var out *store.Comment
	err := s.write(ctx, a, func(t *wtx) error {
		c, err := store.CommentByID(ctx, t.Tx, commentID)
		if err != nil {
			return err
		}
		if c.AuthorID != a.UserID {
			return errForbidden
		}
		if err := store.UpdateComment(ctx, t.Tx, commentID, body, t.now); err != nil {
			return err
		}
		it, err := store.ItemByID(ctx, t.Tx, c.ItemID)
		if err != nil {
			return err
		}
		updated, err := store.CommentByID(ctx, t.Tx, commentID)
		if err != nil {
			return err
		}
		out = &updated
		return t.log(ctx, store.Event{Kind: store.EvCommentUpdated, Entity: "comment",
			EntityID: commentID, ItemID: c.ItemID, TeamID: it.TeamID})
	})
	return out, err
}

func (s *Service) DeleteComment(ctx context.Context, a Actor, commentID int64) error {
	if err := requireUser(a); err != nil {
		return err
	}
	return s.write(ctx, a, func(t *wtx) error {
		c, err := store.CommentByID(ctx, t.Tx, commentID)
		if err != nil {
			return err
		}
		if c.AuthorID != a.UserID && !a.IsAdmin {
			return errForbidden
		}
		it, err := store.ItemByID(ctx, t.Tx, c.ItemID)
		if err != nil {
			return err
		}
		if err := store.DeleteComment(ctx, t.Tx, commentID); err != nil {
			return err
		}
		return t.log(ctx, store.Event{Kind: store.EvCommentDeleted, Entity: "comment",
			EntityID: commentID, ItemID: c.ItemID, TeamID: it.TeamID})
	})
}

func (s *Service) ListComments(ctx context.Context, a Actor, itemID, afterID int64, limit int) ([]store.Comment, error) {
	if err := requireUser(a); err != nil {
		return nil, err
	}
	c, err := store.ListComments(ctx, s.DB.R, itemID, afterID, clampLimit(limit))
	return list(c, mapErr(err))
}
