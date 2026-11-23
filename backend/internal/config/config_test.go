package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{"DGS_ENV", "DGS_LISTEN", "DGS_ALLOWED_ORIGINS", "DGS_LOG_FORMAT", "DGS_LOG_LEVEL",
		"DGS_DB_MAX_CONNS", "DGS_DB_STATEMENT_TIMEOUT"} {
		t.Setenv(k, "")
	}
	// Every case has a database unless it says otherwise.
	t.Setenv("DGS_DATABASE_URL", "postgres://dgs@db/dgs")
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
