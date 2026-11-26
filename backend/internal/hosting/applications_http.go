package hosting

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/notify"
)

const (
	msgNoStatus      = "This status link is not valid."
	msgNoApplication = "Application not found."
	msgDecided       = "This application has already been decided."
	msgTooManyApply  = "Too many applications sent. Please try again later."
)

// Receipt is the front end's HostApplicationReceipt. Token is the status
// link's secret: the applicant gets it by email, and the client only in
// demo mode.
type Receipt struct {
	Application Application `json:"application"`
	Token       string      `json:"token"`
}

func (m *Module) questions(w http.ResponseWriter, r *http.Request) error {
	list, err := Questions(r.Context(), m.db)
	if err != nil {
		return err
	}
	if list == nil {
		list = []Question{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (m *Module) apply(w http.ResponseWriter, r *http.Request) error {
	var in Input
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if m.cfg.Limits != nil {
		keys := map[string]int{"host-apply:address:" + m.cfg.Limits.ClientAddress(r): AppliesPerAddress, "host-apply": AppliesOverall}
		if email := strings.ToLower(strings.TrimSpace(in.ContactEmail)); email != "" {
			keys["host-apply:email:"+email] = AppliesPerEmail
		}
		if err := m.cfg.Limits.Limits(r.Context(), msgTooManyApply, keys); err != nil {
			return err
		}
	}
	a, token, fields, err := Apply(r.Context(), m.db, m.now(), in)
	if err != nil {
		return err
	}
	if err := httpx.CheckFields(fields); err != nil {
		return err
	}
	subject, text := StatusText(a, strings.TrimRight(m.cfg.FrontEnd, "/")+"/host/status/"+token)
	if err := m.cfg.Email.Send(r.Context(), notify.Mail{To: a.ContactEmail, Subject: subject, Text: text}); err != nil {
		// The application stands; the applicant can apply again for a link.
		slog.ErrorContext(r.Context(), "sending a host application's status link failed", "application", a.ID, "err", err)
	}
	out := Receipt{Application: a}
	if m.cfg.ShowTokens {
		out.Token = token
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (m *Module) status(w http.ResponseWriter, r *http.Request) error {
	a, err := ByToken(r.Context(), m.db, r.PathValue("token"))
	if errors.Is(err, ErrNoApplication) {
		return httpx.NotFound(msgNoStatus)
	}
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, a)
	return nil
}

func (m *Module) applications(w http.ResponseWriter, r *http.Request) error {
	list, err := Applications(r.Context(), m.db)
	if err != nil {
		return err
	}
	if list == nil {
		list = []Application{}
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

func (m *Module) decide(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("applicationId"))
	if err != nil {
		return httpx.NotFound(msgNoApplication)
	}
	var in struct {
		Accept *bool `json:"accept"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		return err
	}
	if in.Accept == nil {
		return httpx.FieldError(httpx.CodeInvalidInput, "accept", "Accept or decline the application.")
	}
	u, _ := accounts.CurrentUser(r.Context())
	a, err := Decide(r.Context(), m.db, m.now(), id, u.ID, *in.Accept)
	switch {
	case errors.Is(err, ErrNoApplication):
		return httpx.NotFound(msgNoApplication)
	case errors.Is(err, ErrDecided):
		return httpx.New(httpx.CodeConflict, msgDecided)
	case err != nil:
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, a)
	return nil
}
