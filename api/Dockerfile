# syntax=docker/dockerfile:1
FROM golang:1.27.3-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    for bin in api worker migrate reindex; do \
      CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "/out/newsp-$bin" "./cmd/$bin"; \
    done

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/ /
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/newsp-api"]
