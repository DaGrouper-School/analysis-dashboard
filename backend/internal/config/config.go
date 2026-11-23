// Package config reads the service's configuration from the environment.
// docs/development.md lists every variable and its default.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

// Environments, from least to most protected. Development conveniences,
// such as a default front-end origin on localhost, exist only in the first
// two (docs/design/accounts-and-sign-in.md, "Environments").
const (
	Development = "development"
	Test        = "test"
	Staging     = "staging"
	Production  = "production"
)

var environments = []string{Development, Test, Staging, Production}

// defaultOrigin is the front end's development server.
const defaultOrigin = "http://localhost:5173"

// Config is the service's configuration.
type Config struct {
	// Environment is development, test, staging or production. Required.
	Environment string
	// Listen is the address to serve on, such as :8090.
	Listen string
	// AllowedOrigins are the front-end origins allowed to call with the
	// session cookie (CORS) and to send writes (the origin check).
	AllowedOrigins []string
	// LogFormat is json or text; LogLevel is debug, info, warn or error.
	LogFormat string
	LogLevel  string
}

// Load reads the configuration and refuses unsafe combinations: every
// problem found is reported at once.
func Load() (Config, error) {
	c := Config{
		Environment: os.Getenv("DGS_ENV"),
		Listen:      env("DGS_LISTEN", ":8090"),
		LogLevel:    env("DGS_LOG_LEVEL", "info"),
	}
	if !slices.Contains(environments, c.Environment) {
		return c, fmt.Errorf("DGS_ENV must be one of %s, not %q", strings.Join(environments, ", "), c.Environment)
	}
	logFormat := "json"
	if c.Local() {
		logFormat = "text"
	}
	c.LogFormat = env("DGS_LOG_FORMAT", logFormat)

	var errs []error
	if c.LogFormat != "json" && c.LogFormat != "text" {
		errs = append(errs, fmt.Errorf("DGS_LOG_FORMAT must be json or text, not %q", c.LogFormat))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("DGS_LOG_LEVEL must be debug, info, warn or error, not %q", c.LogLevel))
	}
	origins, err := c.origins(os.Getenv("DGS_ALLOWED_ORIGINS"))
	if err != nil {
		errs = append(errs, err)
	}
	c.AllowedOrigins = origins
	return c, errors.Join(errs...)
}

// origins reads DGS_ALLOWED_ORIGINS. Outside development and test it must be
// set, and every origin must use HTTPS: the session cookie is only ever
// sent over HTTPS there.
func (c Config) origins(v string) ([]string, error) {
	if v == "" {
		if c.Local() {
			return []string{defaultOrigin}, nil
		}
		return nil, fmt.Errorf("DGS_ALLOWED_ORIGINS must be set in %s", c.Environment)
	}
	var out []string
	for o := range strings.SplitSeq(v, ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Host == "" || u.Path != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, fmt.Errorf("DGS_ALLOWED_ORIGINS: %q is not an origin such as https://app.example", o)
		}
		if !c.Local() && u.Scheme != "https" {
			return nil, fmt.Errorf("DGS_ALLOWED_ORIGINS: %q must use https in %s", o, c.Environment)
		}
		out = append(out, o)
	}
	if len(out) == 0 {
		return nil, errors.New("DGS_ALLOWED_ORIGINS names no origin")
	}
	return out, nil
}

// Local reports whether the service runs for development or tests.
func (c Config) Local() bool { return c.Environment == Development || c.Environment == Test }

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
