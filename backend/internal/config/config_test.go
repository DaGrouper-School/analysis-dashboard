package config

import (
	"strings"
	"testing"
	"time"

	"github.com/dagrouper-school/analysis-dashboard/backend/internal/accounts"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"DGS_ENV", "DGS_LISTEN", "DGS_ALLOWED_ORIGINS", "DGS_LOG_FORMAT", "DGS_LOG_LEVEL",
		"DGS_DB_MAX_CONNS", "DGS_DB_STATEMENT_TIMEOUT", "DGS_SAM_AUDIENCE", "DGS_SIGNING_KEYS_FILE", "DGS_RECONCILE_INTERVAL", "DGS_SESSION_TTL", "DGS_DEMO", "DGS_SMS_SENDS_PER_HOUR", "DGS_CLIENT_IP_HEADER", "DGS_PLACES_API_KEY",
		"DGS_SMTP_HOST", "DGS_SMTP_PORT", "DGS_SMTP_SECURITY", "DGS_SMTP_USERNAME", "DGS_SMTP_PASSWORD", "DGS_SMTP_FROM"} {
		t.Setenv(k, "")
	}
	// Every case has a database, SAM and signing keys unless it says otherwise.
	t.Setenv("DGS_DATABASE_URL", "postgres://dgs@db/dgs")
	t.Setenv("DGS_SAM_URL", "https://sam.example")
	t.Setenv("DGS_SAM_TENANT_FILE", "/run/secrets/tenant.json")
	t.Setenv("DGS_SAM_ISSUER", "https://dgs.example")
	t.Setenv("DGS_SIGNING_KEYS_FILE", "/run/secrets/keys.json")
	t.Setenv("DGS_PUBLIC_URL", "https://dgs.example")
	t.Setenv("DGS_GOOGLE_CLIENT_ID", "dgs.apps.googleusercontent.com")
	t.Setenv("DGS_PLACES_API_KEY", "places-key")
	t.Setenv("DGS_SMTP_HOST", "smtp.example")
	t.Setenv("DGS_SMTP_FROM", "DaGrouper School <no-reply@dgs.example>")
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestLoad(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "https://a.example/, https://b.example"})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Local() || c.LogFormat != "json" || c.LogLevel != "info" || c.Listen != ":8090" ||
		c.DB.MaxConns != defaultMaxConns || c.DB.StatementTimeout != defaultStatementTimeout || c.DatabaseURL != "postgres://dgs@db/dgs" {
		t.Fatalf("staging defaults: %+v", c)
	}
	if strings.Join(c.AllowedOrigins, " ") != "https://a.example https://b.example" {
		t.Fatalf("origins: %v", c.AllowedOrigins)
	}
}

func TestLoadDevelopmentDefaults(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "development"})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Local() || c.LogFormat != "text" || strings.Join(c.AllowedOrigins, " ") != defaultOrigin {
		t.Fatalf("development defaults: %+v", c)
	}
}

func TestLoadRefuses(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"no environment", nil, "DGS_ENV"},
		{"unknown environment", map[string]string{"DGS_ENV": "prod"}, "DGS_ENV"},
		{"no origins in production", map[string]string{"DGS_ENV": "production"}, "DGS_ALLOWED_ORIGINS"},
		{"http origin in staging", map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "http://a.example"}, "https"},
		{"not an origin", map[string]string{"DGS_ENV": "test", "DGS_ALLOWED_ORIGINS": "https://a.example/app"}, "not an origin"},
		{"only commas", map[string]string{"DGS_ENV": "test", "DGS_ALLOWED_ORIGINS": ", ,"}, "no origin"},
		{"bad log format", map[string]string{"DGS_ENV": "test", "DGS_LOG_FORMAT": "xml"}, "DGS_LOG_FORMAT"},
		{"bad log level", map[string]string{"DGS_ENV": "test", "DGS_LOG_LEVEL": "loud"}, "DGS_LOG_LEVEL"},
		{"no database", map[string]string{"DGS_ENV": "test", "DGS_DATABASE_URL": ""}, "DGS_DATABASE_URL"},
		{"no connections", map[string]string{"DGS_ENV": "test", "DGS_DB_MAX_CONNS": "0"}, "DGS_DB_MAX_CONNS"},
		{"a bad pool size", map[string]string{"DGS_ENV": "test", "DGS_DB_MAX_CONNS": "many"}, "DGS_DB_MAX_CONNS"},
		{"no statement timeout", map[string]string{"DGS_ENV": "test", "DGS_DB_STATEMENT_TIMEOUT": "0s"}, "DGS_DB_STATEMENT_TIMEOUT"},
		{"no SAM", map[string]string{"DGS_ENV": "test", "DGS_SAM_URL": ""}, "DGS_SAM_URL"},
		{"not a URL for SAM", map[string]string{"DGS_ENV": "test", "DGS_SAM_URL": "sam:8080"}, "DGS_SAM_URL"},
		{"SAM over http in staging", map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_SAM_URL": "http://sam.example"}, "https"},
		{"no tenant file", map[string]string{"DGS_ENV": "test", "DGS_SAM_TENANT_FILE": ""}, "DGS_SAM_TENANT_FILE"},
		{"no issuer", map[string]string{"DGS_ENV": "test", "DGS_SAM_ISSUER": ""}, "DGS_SAM_ISSUER"},
		{"reconciling too often", map[string]string{"DGS_ENV": "test", "DGS_RECONCILE_INTERVAL": "10s"}, "DGS_RECONCILE_INTERVAL"},
		{"no public URL in staging", map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "https://a.example", "DGS_PUBLIC_URL": ""}, "DGS_PUBLIC_URL"},
		{"an http public URL in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_PUBLIC_URL": "http://dgs.example"}, "https"},
		{"not a public URL", map[string]string{"DGS_ENV": "test", "DGS_PUBLIC_URL": "dgs"}, "DGS_PUBLIC_URL"},
		{"no Google client in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_GOOGLE_CLIENT_ID": ""}, "DGS_GOOGLE_CLIENT_ID"},
		{"a short session", map[string]string{"DGS_ENV": "test", "DGS_SESSION_TTL": "10m"}, "DGS_SESSION_TTL"},
		{"demo in staging", map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "https://a.example", "DGS_DEMO": "true"}, "DGS_DEMO"},
		{"demo in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example", "DGS_DEMO": "true"}, "DGS_DEMO"},
		{"demo half on", map[string]string{"DGS_ENV": "test", "DGS_DEMO": "yes"}, "DGS_DEMO"},
		{"no SMS at all", map[string]string{"DGS_ENV": "test", "DGS_SMS_SENDS_PER_HOUR": "0"}, "DGS_SMS_SENDS_PER_HOUR"},
		{"a bad SMS limit", map[string]string{"DGS_ENV": "test", "DGS_SMS_SENDS_PER_HOUR": "lots"}, "DGS_SMS_SENDS_PER_HOUR"},
		{"not a header", map[string]string{"DGS_ENV": "test", "DGS_CLIENT_IP_HEADER": "X Forwarded"}, "DGS_CLIENT_IP_HEADER"},
		{"no places key in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_PLACES_API_KEY": ""}, "DGS_PLACES_API_KEY"},
		{"a bad reconcile interval", map[string]string{"DGS_ENV": "test", "DGS_RECONCILE_INTERVAL": "often"}, "DGS_RECONCILE_INTERVAL"},
		{"no SMTP server in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_SMTP_HOST": ""}, "DGS_SMTP_HOST"},
		{"SMTP in the clear in staging", map[string]string{"DGS_ENV": "staging", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_SMTP_SECURITY": "none"}, "DGS_SMTP_SECURITY"},
		{"unknown SMTP security", map[string]string{"DGS_ENV": "test", "DGS_SMTP_SECURITY": "ssl"}, "DGS_SMTP_SECURITY"},
		{"a bad SMTP port", map[string]string{"DGS_ENV": "test", "DGS_SMTP_PORT": "70000"}, "DGS_SMTP_PORT"},
		{"an SMTP user without a password", map[string]string{"DGS_ENV": "test", "DGS_SMTP_USERNAME": "dgs"}, "DGS_SMTP_PASSWORD"},
		{"no sender", map[string]string{"DGS_ENV": "test", "DGS_SMTP_FROM": ""}, "DGS_SMTP_FROM"},
		{"not a sender", map[string]string{"DGS_ENV": "test", "DGS_SMTP_FROM": "DaGrouper School"}, "DGS_SMTP_FROM"},
		{"no signing keys in production", map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
			"DGS_SIGNING_KEYS_FILE": ""}, "DGS_SIGNING_KEYS_FILE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, tc.env)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to name %s", err, tc.want)
			}
		})
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "production", "DGS_LOG_FORMAT": "xml"})
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DGS_LOG_FORMAT") || !strings.Contains(err.Error(), "DGS_ALLOWED_ORIGINS") {
		t.Fatalf("err = %v, want both problems", err)
	}
}

func TestLoadPool(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "test", "DGS_DB_MAX_CONNS": "4", "DGS_DB_STATEMENT_TIMEOUT": "5s"})
	c, err := Load()
	if err != nil || c.DB.MaxConns != 4 || c.DB.StatementTimeout.Seconds() != 5 {
		t.Fatalf("pool %+v, %v", c.DB, err)
	}
}

func TestLoadSAM(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "development", "DGS_SAM_URL": "http://sam:8080", "DGS_SIGNING_KEYS_FILE": ""})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.SAM.URL != "http://sam:8080" || c.SAM.Audience != "sam" || c.SAM.Issuer != "https://dgs.example" || c.SigningKeysFile != "" ||
		c.ReconcileInterval != defaultReconcileInterval {
		t.Fatalf("SAM in development: %+v, keys %q", c.SAM, c.SigningKeysFile)
	}
}

func TestLoadSignIn(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "development", "DGS_PUBLIC_URL": "", "DGS_GOOGLE_CLIENT_ID": "", "DGS_DEMO": "true"})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicURL != defaultPublicURL || c.Accounts.SecureCookies || !c.Accounts.Demo || c.Accounts.SessionTTL != accounts.DefaultSessionTTL {
		t.Fatalf("sign-in in development: %s %+v", c.PublicURL, c.Accounts)
	}
	if c.Accounts.SMSSendsPerHour != accounts.DefaultSendsPerHour || c.Accounts.ClientIPHeader != "" {
		t.Fatalf("phone limits in development: %+v", c.Accounts)
	}
	setEnv(t, map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example", "DGS_SESSION_TTL": "48h",
		"DGS_SMS_SENDS_PER_HOUR": "50", "DGS_CLIENT_IP_HEADER": "X-Forwarded-For"})
	if c, err = Load(); err != nil || !c.Accounts.SecureCookies || c.Accounts.Demo || c.Accounts.SessionTTL != 48*time.Hour ||
		c.Accounts.GoogleClientID == "" || c.Accounts.SMSSendsPerHour != 50 || c.Accounts.ClientIPHeader != "X-Forwarded-For" {
		t.Fatalf("sign-in in production: %+v, %v", c.Accounts, err)
	}
}

func TestOfflinePlacesInDevelopment(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "development", "DGS_PLACES_API_KEY": ""})
	if c, err := Load(); err != nil || c.Places.APIKey != "" {
		t.Fatalf("places in development without a key: %+v, %v", c.Places, err)
	}
}

func TestLoadSMTP(t *testing.T) {
	setEnv(t, map[string]string{"DGS_ENV": "production", "DGS_ALLOWED_ORIGINS": "https://a.example",
		"DGS_SMTP_USERNAME": "dgs", "DGS_SMTP_PASSWORD": "secret"})
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.SMTP.Host != "smtp.example" || c.SMTP.Port != 587 || c.SMTP.Security != "starttls" || c.SMTP.Username != "dgs" ||
		c.SMTP.From != "DaGrouper School <no-reply@dgs.example>" {
		t.Fatalf("SMTP in production: %+v", c.SMTP)
	}
	setEnv(t, map[string]string{"DGS_ENV": "test", "DGS_SMTP_SECURITY": "tls"})
	if c, err := Load(); err != nil || c.SMTP.Port != 465 {
		t.Fatalf("implicit TLS: port %d, %v", c.SMTP.Port, err)
	}
	setEnv(t, map[string]string{"DGS_ENV": "development", "DGS_SMTP_SECURITY": "none", "DGS_SMTP_PORT": "1025"})
	if c, err := Load(); err != nil || c.SMTP.Port != 1025 {
		t.Fatalf("a local catcher: port %d, %v", c.SMTP.Port, err)
	}
	setEnv(t, map[string]string{"DGS_ENV": "development", "DGS_SMTP_HOST": "", "DGS_SMTP_FROM": ""})
	if c, err := Load(); err != nil || c.SMTP.Host != "" {
		t.Fatalf("development without a server: %+v, %v", c.SMTP, err)
	}
}
