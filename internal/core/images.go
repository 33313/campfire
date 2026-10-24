package core

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/33313/campfire/internal/store"
)

// A picture is pasted into a description or a comment as Markdown naming its
// address (DEV-40). Any authenticated user may upload one, as anyone may
// comment, and read one, as anyone may read every item.
//
// An upload writes no event: a picture changes nothing until a description or
// a comment shows it, and that edit is the event (SPEC.md §10).
//
// ponytail: a picture pasted and then never saved stays in the database. If
// they pile up, reap the ones no description or comment names, lazily on an
// upload.

// imageTypes are the types a picture may be. The bytes decide which it is,
// not what the client says it sent.
var imageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// UploadImage stores a picture of at most max bytes and returns its id. A max
// of 0 means the instance takes none.
func (s *Service) UploadImage(ctx context.Context, a Actor, data []byte, max int64) (int64, error) {
	if err := requireUser(a); err != nil {
		return 0, err
	}
	if max == 0 {
		return 0, invalidField("image", "picture uploads are off on this instance")
	}
	if int64(len(data)) > max {
		return 0, invalidField("image", "a picture is at most %s", size(max))
	}
	kind := http.DetectContentType(data)
	if !imageTypes[kind] {
		return 0, invalidField("image", "a picture is a PNG, JPEG, GIF or WebP")
	}
	var id int64
	err := s.DB.Write(ctx, func(tx *sql.Tx) (err error) {
		id, err = store.CreateImage(ctx, tx, a.UserID, store.Image{ContentType: kind, Data: data}, s.Clock())
		return err
	})
	return id, mapErr(err)
}

func (s *Service) Image(ctx context.Context, a Actor, id int64) (store.Image, error) {
	if err := requireUser(a); err != nil {
		return store.Image{}, err
	}
	img, err := store.ImageByID(ctx, s.DB.R, id)
	return img, mapErr(err)
}

// size says a byte count the way a person sets it: in whole megabytes where
// it is one.
func size(n int64) string {
	if n%(1<<20) == 0 {
		return strconv.FormatInt(n>>20, 10) + "MB"
	}
	return strconv.FormatInt(n>>10, 10) + "KB"
}
