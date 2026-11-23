package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/httpx"
)

// The profile and onboarding (docs/design/accounts-and-sign-in.md, "Profile
// and onboarding"). Profile, phone and address stay in this service; SAM
// holds only the participant ID, and minor reaches it through the user's
// outbox entity.

// The answers the profile can be refused with.
var (
	errEmailInUse    = httpx.FieldError(httpx.CodeConflict, "email", "That email is already in use.")
	errPhoneInUse    = httpx.FieldError(httpx.CodeConflict, "phone", "That phone number belongs to another account.")
	errPhoneTaken    = httpx.FieldError(httpx.CodeInvalidInput, "phone", "That phone number belongs to another account.")
	errOtherAccount  = httpx.FieldError(httpx.CodeCodeInvalid, "code", "This code was issued for another account.")
	errMinorIsPublic = httpx.Forbidden("Contributors under 18 cannot have a public profile.")
)

// emailForm is what the front end's isValidEmail accepts.
var emailForm = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// ProfileInput is the front end's ProfileInput.
type ProfileInput struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

// UpdateProfile sets the user's name and email. An email another account
// holds verified is refused; the user's own Google email stays verified
// while it is unchanged, and any other they type is not.
func (m *Module) UpdateProfile(ctx context.Context, id uuid.UUID, in ProfileInput) (User, error) {
	name, email := strings.TrimSpace(in.FullName), strings.ToLower(strings.TrimSpace(in.Email))
	fields := map[string]string{}
	if len([]rune(name)) < 2 {
		fields["fullName"] = "Enter your full name."
	}
	if !emailForm.MatchString(email) {
		fields["email"] = "Enter a valid email address."
	}
	if err := httpx.CheckFields(fields); err != nil {
		return User{}, err
	}
	return m.change(ctx, id, func(tx pgx.Tx, cur User) error {
		owner, taken, err := userWithEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		if taken && owner != id {
			return errEmailInUse
		}
		verified := cur.EmailVerified && cur.Email != nil && strings.EqualFold(*cur.Email, email)
		return setProfile(ctx, tx, id, name, email, verified, m.now())
	})
}

// Address is the front end's Address.
type Address struct {
	Street  string   `json:"street"`
	Suite   string   `json:"suite,omitempty"`
	City    string   `json:"city"`
	State   string   `json:"state"`
	Zip     string   `json:"zip"`
	Country string   `json:"country"`
	PlaceID string   `json:"placeId,omitempty"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
}

// UpdateAddress sets the user's address: a street, a city and a country
// at least.
func (m *Module) UpdateAddress(ctx context.Context, id uuid.UUID, a Address) (User, error) {
	fields := map[string]string{}
	if strings.TrimSpace(a.Street) == "" {
		fields["street"] = "Enter a street address."
	}
	if strings.TrimSpace(a.City) == "" {
		fields["city"] = "Enter a city."
	}
	if strings.TrimSpace(a.Country) == "" {
		fields["country"] = "Enter a country."
	}
	if a.Lat != nil && (*a.Lat < -90 || *a.Lat > 90) || a.Lng != nil && (*a.Lng < -180 || *a.Lng > 180) {
		fields["street"] = "That place is not on the map."
	}
	if err := httpx.CheckFields(fields); err != nil {
		return User{}, err
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return User{}, err
	}
	return m.change(ctx, id, func(tx pgx.Tx, _ User) error { return setAddress(ctx, tx, id, raw, m.now()) })
}

// StartPhoneVerification sends the user a code to verify a number with,
// from the client at addr, with the codes and limits of phone sign-in. A
// number another account has verified is refused.
func (m *Module) StartPhoneVerification(ctx context.Context, id uuid.UUID, raw, addr string) (PhoneChallenge, error) {
	if phone := NormalizePhone(raw); phone != "" {
		owner, taken, err := userWithPhone(ctx, m.db, phone)
		if err != nil {
			return PhoneChallenge{}, err
		}
		if taken && owner != id {
			return PhoneChallenge{}, errPhoneInUse
		}
	}
	return m.RequestCode(ctx, raw, PurposeVerifyPhone, &id, addr)
}

// ConfirmPhoneVerification checks the code requestID names, sent to the
// user, and records its number as theirs and verified.
func (m *Module) ConfirmPhoneVerification(ctx context.Context, id uuid.UUID, requestID, code, addr string) (User, error) {
	c, err := m.CheckCode(ctx, requestID, code, PurposeVerifyPhone, addr)
	if err != nil {
		return User{}, err
	}
	if c.user == nil || *c.user != id {
		return User{}, errOtherAccount
	}
	u, err := m.change(ctx, id, func(tx pgx.Tx, _ User) error { return setVerifiedPhone(ctx, tx, id, c.phone, m.now()) })
	if isUniqueViolation(err) {
		// Another account verified the number since the code was sent.
		return User{}, errPhoneTaken
	}
	return u, err
}

// SetPublicProfile shows or hides the user's name and role next to their
// participant ID on contributor pages. A minor never has one.
func (m *Module) SetPublicProfile(ctx context.Context, id uuid.UUID, show bool) (User, error) {
	return m.change(ctx, id, func(tx pgx.Tx, cur User) error {
		if show && cur.Minor != nil && *cur.Minor {
			return errMinorIsPublic
		}
		return setPublicProfile(ctx, tx, id, show, m.now())
	})
}

// MarkMinor marks the user as a minor, with their guardian's consent
// pending and no public profile, and queues minor=true for SAM. A user
// already a minor keeps their consent as it is.
func (m *Module) MarkMinor(ctx context.Context, id uuid.UUID) (User, error) {
	return m.change(ctx, id, func(tx pgx.Tx, cur User) error {
		if cur.Minor != nil && *cur.Minor {
			return nil
		}
		now := m.now()
		if _, err := SetMinor(ctx, tx, now, id, true); err != nil {
			return err
		}
		if _, err := SetConsent(ctx, tx, now, id, ConsentPending); err != nil {
			return err
		}
		return setPublicProfile(ctx, tx, id, false, now)
	})
}

// CompleteOnboarding finishes onboarding once the user has a name, an
// email, a verified phone and an address. For an adult it queues
// minor=false for SAM, which until then treats them as a minor.
func (m *Module) CompleteOnboarding(ctx context.Context, id uuid.UUID) (User, error) {
	return m.change(ctx, id, func(tx pgx.Tx, cur User) error {
		var missing []string
		if cur.FullName == "" {
			missing = append(missing, "name")
		}
		if cur.Email == nil || *cur.Email == "" {
			missing = append(missing, "email")
		}
		if !cur.PhoneVerified {
			missing = append(missing, "verified phone")
		}
		if cur.Address == nil {
			missing = append(missing, "address")
		}
		if len(missing) > 0 {
			return httpx.New(httpx.CodeInvalidInput, fmt.Sprintf("Still needed: %s.", strings.Join(missing, ", ")))
		}
		now := m.now()
		if cur.Minor == nil || !*cur.Minor {
			if _, err := SetMinor(ctx, tx, now, id, false); err != nil {
				return err
			}
		}
		return setOnboardingCompleted(ctx, tx, id, now)
	})
}

// change runs one change to the user in a transaction, with their row
// locked, and returns them as it leaves them.
func (m *Module) change(ctx context.Context, id uuid.UUID, apply func(tx pgx.Tx, cur User) error) (User, error) {
	var u User
	err := db.InTx(ctx, m.db, func(tx pgx.Tx) error {
		cur, err := lockUser(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := apply(tx, cur); err != nil {
			return err
		}
		u, err = getUser(ctx, tx, id)
		return err
	})
	return u, err
}
