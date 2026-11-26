package classrooms

import (
	"net/http"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

const msgSharing = "Only the institution head can change how the classroom shares its data."

// updateSharing sets a classroom's sharing settings and data terms: the
// head's, and a platform admin's.
func (m *Module) updateSharing(w http.ResponseWriter, r *http.Request) error {
	c, _, can, err := m.visible(r)
	if err != nil {
		return err
	}
	if !can.ManageSharing {
		return httpx.Forbidden(msgSharing)
	}
	var in SharingInput
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if err := httpx.CheckFields(CheckSharing(in)); err != nil {
		return err
	}
	u, _ := accounts.CurrentUser(r.Context())
	c, err = SetSharing(r.Context(), m.db, m.now(), c, u.ID, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, SharingInput{Sharing: c.Sharing, Terms: c.Terms})
	return nil
}
