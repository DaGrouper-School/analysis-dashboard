module github.com/dagrouper-school/analysis-dashboard/backend

go 1.27.0

toolchain go1.27.4

require (
	github.com/dagrouper-school/sam v0.0.0-20261126072151-9239495216fe
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.13.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	go.yaml.in/yaml/v3 v3.0.6
)

require (
	github.com/go-chi/chi/v5 v5.5.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

tool github.com/dagrouper-school/sam/cmd/sam
