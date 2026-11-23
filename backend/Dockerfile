FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
# The module and build caches outlive the image, so later builds download
# and compile only what changed.
RUN --mount=type=cache,target=/go/pkg/mod go mod download
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
