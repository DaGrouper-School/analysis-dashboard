// Package settings holds the platform's settings, one checked value per
// key in platform_settings, and the configuration change log every change
// to the platform's configuration is written to
// (docs/design/registry-and-content.md#platform-configuration).
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/jsoncheck"
)

// The settings' keys.
const (
	KeyCoreParameters = "core_parameters"
	KeyDefaultTerms   = "default_terms"
	KeyPrivacy        = "privacy"
	KeyFuture         = "future_observations"
)

// Key is a setting's key and what its value holds.
type Key struct {
	Name  string
	About string
}

// setting is a key with how its value is checked.
type setting struct {
	Key
	check func([]byte) (any, []string)
}

// keys are the settings, in a fixed order.
var keys = []setting{
	{Key{KeyCoreParameters, "which registry values feed the shared core readings, the rain reference and tolerance"}, checkAs[CoreParameters]},
	{Key{KeyDefaultTerms, "the data terms a new classroom starts with"}, checkAs[DataTerms]},
	{Key{KeyPrivacy, "the decimals a minor's positions are rounded to"}, checkAs[Privacy]},
	{Key{KeyFuture, "whether observations dated in the future are flagged and stored, or refused"}, checkAs[Future]},
}

// Keys are the settings' keys, in a fixed order.
func Keys() []Key {
	out := make([]Key, len(keys))
	for i, k := range keys {
		out[i] = k.Key
	}
	return out
}

// ErrUnknownKey is the answer for a key that is not a setting.
var ErrUnknownKey = errors.New("not a setting")

// ErrNotSet is the answer for a setting with no value yet.
var ErrNotSet = errors.New("no value is set; dgs seed sets every setting")

// Entry is a setting's stored value.
type Entry struct {
	Key   string
	Value json.RawMessage
	// Seeded is true while the seed wrote the value last.
	Seeded    bool
	UpdatedAt time.Time
}

// Get is the value stored for key.
func Get(ctx context.Context, q db.Querier, key string) (Entry, error) {
	if _, err := keyOf(key); err != nil {
		return Entry{}, err
	}
	e, err := get(ctx, q, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, fmt.Errorf("%s: %w", key, ErrNotSet)
	}
	return e, err
}

// All are the stored settings, by key; a key with no value is left out.
func All(ctx context.Context, q db.Querier) (map[string]Entry, error) { return all(ctx, q) }

// Check is raw, checked as key's value, in the form it is stored in.
// Every problem found is reported at once.
func Check(key string, raw []byte) (json.RawMessage, error) {
	k, err := keyOf(key)
	if err != nil {
		return nil, err
	}
	v, problems := k.check(raw)
	if err := jsoncheck.Refuse(key, problems); err != nil {
		return nil, err
	}
	return jsoncheck.Canonical(v)
}

// Put checks raw and stores it as key's value, written by actor, and logs
// the change. It reports whether anything changed.
func Put(ctx context.Context, q db.Querier, now time.Time, actor Actor, key string, raw []byte) (bool, error) {
	value, err := Check(key, raw)
	if err != nil {
		return false, err
	}
	changed := false
	err = db.InTx(ctx, q, func(tx pgx.Tx) error {
		var err error
		if changed, err = put(ctx, tx, now, key, value); err != nil || !changed {
			return err
		}
		return Record(ctx, tx, now, actor, Change{Kind: KindSetting, Subject: key, Summary: "Changed " + key})
	})
	return changed, err
}

func keyOf(name string) (setting, error) {
	i := slices.IndexFunc(keys, func(k setting) bool { return k.Name == name })
	if i < 0 {
		names := make([]string, len(keys))
		for j, k := range keys {
			names[j] = k.Name
		}
		return setting{}, fmt.Errorf("%q: %w; the settings are %s", name, ErrUnknownKey, strings.Join(names, ", "))
	}
	return keys[i], nil
}

// checker is a setting's value: it knows the rules beyond its shape.
type checker interface{ check() []string }

// checkAs decodes raw strictly as a T, then checks it has no blank text
// and keeps T's own rules.
func checkAs[T checker](raw []byte) (any, []string) {
	v, problems := jsoncheck.Decode[T](raw)
	if len(problems) > 0 {
		return nil, problems
	}
	return v, append(v.check(), jsoncheck.Blank(v, "")...)
}
