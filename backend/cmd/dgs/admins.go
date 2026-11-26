package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// adminsCommand lists the platform admins, or makes or unmakes one, as the
// operator's decision. who is a verified email or a user ID.
func adminsCommand(ctx context.Context, out io.Writer, pool *pgxpool.Pool, args []string) error {
	if len(args) == 2 {
		admin := args[0] == "add"
		u, changed, err := accounts.SetAdmin(ctx, pool, time.Now(), settings.Operator, args[1], admin)
		if err != nil {
			return fmt.Errorf("%s: %w; the person signs in once first, then is named by their Google email or user ID", args[1], err)
		}
		switch {
		case !changed && admin:
			_, err = fmt.Fprintf(out, "%s is already a platform admin\n", u.FullName)
		case !changed:
			_, err = fmt.Fprintf(out, "%s is not a platform admin\n", u.FullName)
		case admin:
			_, err = fmt.Fprintf(out, "%s is a platform admin; SAM follows through the outbox\n", u.FullName)
		default:
			_, err = fmt.Fprintf(out, "%s is no longer a platform admin, here at once; SAM follows through the outbox\n", u.FullName)
		}
		return err
	}
	list, err := accounts.Admins(ctx, pool)
	if err != nil {
		return err
	}
	// The tabwriter buffers: a write error shows when it flushes.
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tEMAIL\tUSER\tIN SAM")
	for _, u := range list {
		email := ""
		if u.Email != nil {
			email = *u.Email
		}
		inSAM := "not yet"
		if u.SAMAdmin {
			inSAM = "yes"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", u.FullName, email, u.ID, inSAM)
	}
	return tw.Flush()
}
