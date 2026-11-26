package classrooms

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/roles"
)

func TestCleanEmails(t *testing.T) {
	t.Parallel()
	good, bad := cleanEmails([]string{" Hana@Trinity.example ", "hana@trinity.example", "", "not an email", "b@c.example"})
	if !slices.Equal(good, []string{"hana@trinity.example", "b@c.example"}) || !slices.Equal(bad, []string{"not an email"}) {
		t.Errorf("good %q, bad %q", good, bad)
	}
}

func TestUsable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	by := uuid.New()
	base := Invitation{Status: StatusPending, ExpiresAt: now.Add(time.Hour), InvitedBy: &by}
	for _, c := range []struct {
		name string
		edit func(*Invitation)
		want error
	}{
		{"open", func(*Invitation) {}, nil},
		{"revoked", func(i *Invitation) { i.Status = StatusRevoked }, ErrNoInvitation},
		{"used", func(i *Invitation) { i.Status = StatusAccepted }, ErrUsed},
		{"expired", func(i *Invitation) { i.ExpiresAt = now }, ErrExpired},
		{"its inviter gone", func(i *Invitation) { i.InvitedBy = nil }, ErrOrphaned},
		{"from this service", func(i *Invitation) { i.InvitedBy, i.ByService = nil, true }, nil},
	} {
		i := base
		c.edit(&i)
		if err := i.usable(now); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

func TestInvitationText(t *testing.T) {
	t.Parallel()
	c := Classroom{Name: "Trinity Weather", InstitutionName: "Trinity School"}
	inv := Invitation{Email: "dawit@trinity.example", Role: roles.Facilitator, InvitedByName: "Hana Bekele",
		ExpiresAt: time.Date(2026, 12, 10, 9, 0, 0, 0, time.UTC)}
	subject, text := InvitationText(c, inv, "https://app.example/invite/abc")
	if subject != "You're invited to Trinity Weather" || !strings.Contains(text, "Hana Bekele invited you to join Trinity Weather, a Global Classroom at Trinity School, as facilitator.") ||
		!strings.Contains(text, "https://app.example/invite/abc") || !strings.Contains(text, "10 December 2026") {
		t.Errorf("%q\n%s", subject, text)
	}
}

func TestParseMember(t *testing.T) {
	t.Parallel()
	c, u := uuid.New(), uuid.New()
	gc, gu, err := parseMember(memberEntity(c, u))
	if err != nil || gc != c || gu != u {
		t.Fatalf("%v %v %v", gc, gu, err)
	}
	if _, _, err := parseMember("member:x"); err == nil {
		t.Error("a bad entity parsed")
	}
}

// TestInvitationFollowsInviter: a member's pending invitations to roles
// their new role does not hand on are revoked, and all of them when they
// leave.
func TestInvitationFollowsInviter(t *testing.T) {
	t.Parallel()
	ctx, pool, c := configured(t)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	facilitator := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, created_at, updated_at) VALUES ($1, now(), now())`, facilitator); err != nil {
		t.Fatal(err)
	}
	if err := SetMemberRole(ctx, pool, now, c.ID, facilitator, roles.Facilitator, nil); !errors.Is(err, ErrNoMember) {
		t.Fatalf("a role for no member: %v", err)
	}
	if err := setRole(ctx, pool, now, c, facilitator, roles.Facilitator, nil, false); err != nil {
		t.Fatal(err)
	}
	by := Inviter{UserID: &facilitator, Name: "Dawit"}
	if _, err := Invite(ctx, pool, now, time.Hour, c, []string{"a@b.example", "c@d.example"}, roles.CommunityScientist, by); err != nil {
		t.Fatal(err)
	}
	pending := func() int {
		list, err := PendingIn(ctx, pool, now, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, i := range list {
			if i.InvitedBy != nil && *i.InvitedBy == facilitator {
				n++
			}
		}
		return n
	}
	if pending() != 2 {
		t.Fatalf("pending: %d", pending())
	}
	if err := SetMemberRole(ctx, pool, now, c.ID, facilitator, roles.Researcher, nil); err != nil {
		t.Fatal(err)
	}
	if pending() != 0 {
		t.Errorf("a researcher's invitations to community scientists stayed: %d", pending())
	}
	var takesAway bool
	if err := pool.QueryRow(ctx, `SELECT takes_access_away FROM sam_outbox WHERE entity = $1 ORDER BY version DESC LIMIT 1`,
		memberEntity(c.ID, facilitator)).Scan(&takesAway); err != nil || !takesAway {
		t.Errorf("a narrowed role takes access away: %v, %v", takesAway, err)
	}
	if err := RemoveMember(ctx, pool, now, c.ID, facilitator, nil); err != nil {
		t.Fatal(err)
	}
	if role, _ := RoleOf(ctx, pool, c.ID, facilitator); role != "" {
		t.Errorf("a member who left still has %s", role)
	}
}
