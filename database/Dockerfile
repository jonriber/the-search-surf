FROM postgis/postgis:18-3.6-alpine@sha256:eb2e8b8afd9b0ecee83bc20fd01aca62a5071bada2c0f38763174b653f8eed42

LABEL org.opencontainers.image.title="The Search Database" \
      org.opencontainers.image.description="Non-root PostgreSQL and PostGIS runtime for The Search" \
      org.opencontainers.image.source="https://github.com/jonriber/the-search-surf" \
      org.opencontainers.image.licenses="Apache-2.0"

# The pinned base still ships OpenSSL packages affected by CVE-2026-14456. The
# upstream entry point uses gosu only when it starts as root. Running as the
# image's existing postgres UID/GID removes that privilege transition and its
# otherwise unused Go binary from the runtime attack surface.
USER root
RUN apk add --no-cache --upgrade \
        libcrypto3=3.5.8-r0 \
        libssl3=3.5.8-r0 && \
    rm /usr/local/bin/gosu
USER 70:70
