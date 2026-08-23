# syntax=docker/dockerfile:1.20

ARG GO_IMAGE=golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
FROM ${GO_IMAGE} AS build

WORKDIR /src

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . ./

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/api ./cmd/api

FROM scratch

LABEL org.opencontainers.image.title="The Search API" \
      org.opencontainers.image.description="Go API for explainable, personalized surf recommendations" \
      org.opencontainers.image.source="https://github.com/jonriber/the-search-surf" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/api /api

USER 65532:65532
EXPOSE 8080
ENV HTTP_ADDRESS=:8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/api", "healthcheck"]

ENTRYPOINT ["/api"]
