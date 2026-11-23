FROM golang:1.27-alpine AS build
# SAM's module is private and fetched with git.
RUN apk add --no-cache git
ENV GOPRIVATE=github.com/dagrouper-school/*
WORKDIR /src
COPY go.mod go.sum ./
# The module and build caches outlive the image, so later builds download
# and compile only what changed. The token for SAM's repository is a build
# secret: it reaches this step only, never a layer.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=secret,id=sam_read_token,required=true \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0="url.https://x-access-token:$(cat /run/secrets/sam_read_token)@github.com/dagrouper-school/.insteadOf" \
    GIT_CONFIG_VALUE_0="https://github.com/dagrouper-school/" \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/dgs ./cmd/dgs

FROM alpine:3.22
RUN adduser -D -H dgs
COPY --from=build /out/dgs /usr/local/bin/dgs
USER dgs
EXPOSE 8090
HEALTHCHECK --interval=5s --timeout=3s --retries=10 CMD wget -qO- http://127.0.0.1:8090/healthz >/dev/null || exit 1
ENTRYPOINT ["dgs"]
CMD ["serve"]
