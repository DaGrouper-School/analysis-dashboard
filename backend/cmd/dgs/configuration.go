package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/classrooms"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/hosting"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/registry"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/settings"
)

// settingsCommand lists the settings, prints one, or checks and stores
// one as the operator's.
func settingsCommand(ctx context.Context, out io.Writer, pool *pgxpool.Pool, args []string) error {
	switch {
	case len(args) == 2:
		e, err := settings.Get(ctx, pool, args[1])
		if err != nil {
			return err
		}
		var b bytes.Buffer
		if err := json.Indent(&b, e.Value, "", "  "); err != nil {
			return err
		}
		b.WriteByte('\n')
		_, err = out.Write(b.Bytes())
		return err
	case len(args) == 3:
		raw, err := readFile(args[2])
		if err != nil {
			return err
		}
		changed, err := settings.Put(ctx, pool, time.Now(), settings.Operator, args[1], raw)
		if err != nil {
			return err
		}
		if !changed {
			_, err = fmt.Fprintf(out, "%s is unchanged\n", args[1])
			return err
		}
		_, err = fmt.Fprintf(out, "stored %s; pages show it at once\n", args[1])
		return err
	}
	stored, err := settings.All(ctx, pool)
	if err != nil {
		return err
	}
	// The tabwriter buffers: a write error shows when it flushes.
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "KEY\tSET BY\tSINCE\tHOLDS")
	for _, k := range settings.Keys() {
		by, since := "not set", ""
		if e, ok := stored[k.Name]; ok {
			by, since = "an admin or operator", e.UpdatedAt.UTC().Format(time.RFC3339)
			if e.Seeded {
				by = "the seed"
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", k.Name, by, since, k.About)
	}
	return tw.Flush()
}

// readFile is the file at path, or standard input for -.
func readFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// seed loads the platform's configuration from the seed built into the
// binary, in one transaction under the seed lock, and says what it did
// with each setting and row.
func seed(ctx context.Context, out io.Writer, pool *pgxpool.Pool) error {
	now := time.Now()
	var done []settings.Seeded
	err := db.InTx(ctx, pool, func(tx pgx.Tx) error {
		done = nil
		if err := db.LockTx(ctx, tx, db.SeedLock); err != nil {
			return err
		}
		for _, step := range []func(context.Context, db.Querier, time.Time) ([]settings.Seeded, error){
			settings.SeedSettings, classrooms.SeedLists, hosting.SeedQuestions, registry.SeedOverlay, classrooms.SeedConsent,
		} {
			seeded, err := step(ctx, tx, now)
			done = append(done, seeded...)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, s := range done {
		what := map[settings.Outcome]string{
			settings.Added:     "added",
			settings.Updated:   "updated to the seed's",
			settings.Unchanged: "unchanged",
			settings.Kept:      "kept: changed since by an admin or operator",
			settings.LeftOut:   "left out: the list has rows, so the seed adds none",
		}[s.Outcome]
		if _, err := fmt.Fprintf(out, "%s: %s\n", s.What, what); err != nil {
			return err
		}
	}
	return nil
}

// publishConsent publishes the consent text in file as a new version, as
// the operator's.
func publishConsent(ctx context.Context, out io.Writer, pool *pgxpool.Pool, file string) error {
	raw, err := readFile(file)
	if err != nil {
		return err
	}
	version, err := classrooms.PublishConsent(ctx, pool, time.Now(), settings.Operator, raw)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "published consent text version %d; sheets and links show it at once\n", version)
	return err
}

// configureClassroom sets the classroom with code's stage, region and
// profile fields from file, as the operator's.
func configureClassroom(ctx context.Context, out io.Writer, pool *pgxpool.Pool, code, file string) error {
	raw, err := readFile(file)
	if err != nil {
		return err
	}
	c, err := classrooms.ByCode(ctx, pool, code)
	if err != nil {
		return fmt.Errorf("classroom %s: %w", code, err)
	}
	if err := classrooms.Configure(ctx, pool, time.Now(), settings.Operator, c.ID, raw); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "configured %s; its pages show it at once\n", code)
	return err
}
