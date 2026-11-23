// Package config reads the service's configuration from the environment.
// docs/development.md lists every variable and its default.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/db"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/places"
	"github.com/dagrouper-school/analysis-dashboard/backend/internal/samaccess"
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
	// DatabaseURL is the PostgreSQL connection string. Required.
	DatabaseURL string
	// DB sizes the pool and bounds its statements.
	DB db.Config
	// SAM is how this service reaches SAM. Required.
	SAM samaccess.Config
	// ReconcileInterval is how often the reconciler compares SAM with this
	// database.
	ReconcileInterval time.Duration
	// PublicURL is where the service is reached, such as
	// https://dgs.example; the session cookie is Secure when it is HTTPS.
	PublicURL string
	// Accounts is how people sign in.
	Accounts accounts.Config
	// Places is how places are looked up.
	Places places.Config
	// SigningKeysFile is the key file user tokens are signed with. Required
	// outside development and test; there, without one, a key is generated
	// at start-up and lasts until the process exits.
	SigningKeysFile string
}

// Pool defaults: enough connections for one instance's requests and its
// background work, and a statement bound well above any request's.
const (
	defaultMaxConns         = 10
	defaultStatementTimeout = 30 * time.Second
)

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
	c.DatabaseURL = os.Getenv("DGS_DATABASE_URL")
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DGS_DATABASE_URL must be set: the PostgreSQL connection string"))
	}
	c.DB, err = pool()
	if err != nil {
		errs = append(errs, err)
	}
	c.SAM, err = c.sam()
	if err != nil {
		errs = append(errs, err)
	}
	c.ReconcileInterval = defaultReconcileInterval
	if v := os.Getenv("DGS_RECONCILE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < minReconcileInterval {
			errs = append(errs, fmt.Errorf("DGS_RECONCILE_INTERVAL must be a duration of at least %s, such as 15m, not %q", minReconcileInterval, v))
		}
		c.ReconcileInterval = d
	}
	if err := c.signIn(); err != nil {
		errs = append(errs, err)
	}
	c.Places = places.Config{APIKey: os.Getenv("DGS_PLACES_API_KEY")}
	if c.Places.APIKey == "" && !c.Local() {
		errs = append(errs, fmt.Errorf("DGS_PLACES_API_KEY must be set in %s: the Google Places key address lookup uses; "+
			"the offline list exists only in development and test", c.Environment))
	}
	c.SigningKeysFile = os.Getenv("DGS_SIGNING_KEYS_FILE")
	if c.SigningKeysFile == "" && !c.Local() {
		errs = append(errs, fmt.Errorf("DGS_SIGNING_KEYS_FILE must be set in %s: generated keys exist only in development and test", c.Environment))
	}
	return c, errors.Join(errs...)
}

// The reconciler's interval: each pass reads every user from SAM, and SAM
// audits each attribute read, so a pass is not run more often than a
// minute.
const (
	defaultReconcileInterval = 15 * time.Minute
	minReconcileInterval     = time.Minute
)

// headerName is what an HTTP header name may hold.
var headerName = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// Sign-in defaults.
const (
	defaultPublicURL = "http://localhost:8090"
	minSessionTTL    = time.Hour
)

// signIn reads DGS_PUBLIC_URL, DGS_GOOGLE_CLIENT_ID, DGS_SESSION_TTL and
// DGS_DEMO. Outside development and test the service is reached over HTTPS,
// Google sign-in has a client ID, and demo mode is refused.
func (c *Config) signIn() error {
	var errs []error
	c.PublicURL = os.Getenv("DGS_PUBLIC_URL")
	if c.PublicURL == "" && c.Local() {
		c.PublicURL = defaultPublicURL
	}
	u, err := url.Parse(c.PublicURL)
	switch {
	case c.PublicURL == "":
		errs = append(errs, fmt.Errorf("DGS_PUBLIC_URL must be set in %s: the URL the service is reached at", c.Environment))
	case err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http"):
		errs = append(errs, fmt.Errorf("DGS_PUBLIC_URL must be a URL such as https://dgs.example, not %q", c.PublicURL))
	case !c.Local() && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("DGS_PUBLIC_URL must use https in %s", c.Environment))
	}
	c.Accounts = accounts.Config{GoogleClientID: os.Getenv("DGS_GOOGLE_CLIENT_ID"), SessionTTL: accounts.DefaultSessionTTL,
		SecureCookies: u != nil && u.Scheme == "https"}
	if c.Accounts.GoogleClientID == "" && !c.Local() {
		errs = append(errs, fmt.Errorf("DGS_GOOGLE_CLIENT_ID must be set in %s: the Google OAuth client ID sign-in accepts", c.Environment))
	}
	if v := os.Getenv("DGS_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < minSessionTTL {
			errs = append(errs, fmt.Errorf("DGS_SESSION_TTL must be a duration of at least %s, such as 720h, not %q", minSessionTTL, v))
		}
		c.Accounts.SessionTTL = d
	}
	c.Accounts.SMSSendsPerHour = accounts.DefaultSendsPerHour
	if v := os.Getenv("DGS_SMS_SENDS_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Errorf("DGS_SMS_SENDS_PER_HOUR must be a whole number from 1, not %q", v))
		}
		c.Accounts.SMSSendsPerHour = n
	}
	c.Accounts.ClientIPHeader = os.Getenv("DGS_CLIENT_IP_HEADER")
	if h := c.Accounts.ClientIPHeader; h != "" && !headerName.MatchString(h) {
		errs = append(errs, fmt.Errorf("DGS_CLIENT_IP_HEADER must be a header name, such as X-Forwarded-For, not %q", h))
	}
	switch v := os.Getenv("DGS_DEMO"); v {
	case "", "false":
	case "true":
		if !c.Local() {
			errs = append(errs, fmt.Errorf("DGS_DEMO must not be on in %s: demo sign-in exists only in development and test", c.Environment))
		}
		c.Accounts.Demo = true
	default:
		errs = append(errs, fmt.Errorf("DGS_DEMO must be true or false, not %q", v))
	}
	return errors.Join(errs...)
}

// defaultAudience is the audience this service's user tokens carry for SAM.
const defaultAudience = "sam"

// sam reads DGS_SAM_URL, DGS_SAM_TENANT_FILE, DGS_SAM_ISSUER and
// DGS_SAM_AUDIENCE. Outside development and test SAM is reached over HTTPS.
func (c Config) sam() (samaccess.Config, error) {
	cfg := samaccess.Config{URL: os.Getenv("DGS_SAM_URL"), TenantFile: os.Getenv("DGS_SAM_TENANT_FILE"),
		Issuer: os.Getenv("DGS_SAM_ISSUER"), Audience: env("DGS_SAM_AUDIENCE", defaultAudience)}
	var errs []error
	if u, err := url.Parse(cfg.URL); cfg.URL == "" || err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		errs = append(errs, fmt.Errorf("DGS_SAM_URL must be SAM's URL, such as https://sam.example, not %q", cfg.URL))
	} else if !c.Local() && u.Scheme != "https" {
		errs = append(errs, fmt.Errorf("DGS_SAM_URL must use https in %s", c.Environment))
	}
	if cfg.TenantFile == "" {
		errs = append(errs, errors.New("DGS_SAM_TENANT_FILE must be set: the JSON `sam tenant create -json` printed"))
	}
	if cfg.Issuer == "" {
		errs = append(errs, errors.New("DGS_SAM_ISSUER must be set: the issuer this service was onboarded with"))
	}
	return cfg, errors.Join(errs...)
}

// pool reads DGS_DB_MAX_CONNS and DGS_DB_STATEMENT_TIMEOUT.
func pool() (db.Config, error) {
	cfg := db.Config{MaxConns: defaultMaxConns, StatementTimeout: defaultStatementTimeout}
	var errs []error
	if v := os.Getenv("DGS_DB_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Errorf("DGS_DB_MAX_CONNS must be a whole number from 1, not %q", v))
		}
		cfg.MaxConns = int32(n)
	}
	if v := os.Getenv("DGS_DB_STATEMENT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Millisecond {
			errs = append(errs, fmt.Errorf("DGS_DB_STATEMENT_TIMEOUT must be a duration of at least 1ms, such as 30s, not %q", v))
		}
		cfg.StatementTimeout = d
	}
	return cfg, errors.Join(errs...)
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
