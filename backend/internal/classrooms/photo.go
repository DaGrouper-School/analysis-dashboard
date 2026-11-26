package classrooms

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/media"
)

// The host institution's photo: set by the institution head, kept in the
// media store under its content hash, and named in the profile by its
// address.

// maxPhotoBody bounds a photo request: the largest image as a data URL,
// with room for the rest.
const maxPhotoBody = 2_100_000

// PhotoInput is the front end's InstitutionPhotoInput.
type PhotoInput struct {
	Image *string `json:"image"`
	Alt   string  `json:"alt"`
}

// SetPhoto names the institution's photo and what it shows, or removes
// it when url is nil, and returns the institution as it now is.
func SetPhoto(ctx context.Context, q db.Querier, now time.Time, id uuid.UUID, url *string, alt string) (Institution, error) {
	var out Institution
	err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
		c, err := lock(ctx, tx, id)
		if err != nil {
			return err
		}
		c.Profile.Institution.PhotoURL, c.Profile.Institution.PhotoAlt = url, ""
		if url != nil {
			c.Profile.Institution.PhotoAlt = alt
		}
		out = c.Profile.Institution
		return save(ctx, tx, now, c)
	})
	return out, err
}

func (m *Module) setPhoto(w http.ResponseWriter, r *http.Request) error {
	c, _, can, err := m.visible(r)
	if err != nil {
		return err
	}
	if !can.Edit {
		return httpx.Forbidden("Only the institution head can change the institution's photo.")
	}
	var in PhotoInput
	if err := httpx.DecodeLimit(r, &in, maxPhotoBody); err != nil {
		return err
	}
	var url *string
	alt := strings.TrimSpace(in.Alt)
	if in.Image != nil {
		f := map[string]string{}
		img, err := media.ParseImage(*in.Image)
		var problem media.Problem
		if errors.As(err, &problem) {
			f["image"] = problem.Error()
		} else if err != nil {
			return err
		}
		if len([]rune(alt)) < 3 {
			f["alt"] = "Say what the photo shows."
		}
		if err := httpx.CheckFields(f); err != nil {
			return err
		}
		if m.cfg.Media == nil {
			return errors.New("classrooms: no media store to keep the photo in")
		}
		name, err := m.cfg.Media.Save(r.Context(), media.KindInstitutionPhoto, c.ID, img)
		if err != nil {
			return err
		}
		link := strings.TrimRight(m.cfg.MediaURL, "/") + media.Path + name
		url = &link
	}
	out, err := SetPhoto(r.Context(), m.db, m.now(), c.ID, url, alt)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}
