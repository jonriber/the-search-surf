FROM postgis/postgis:18-3.6-alpine@sha256:c53a483fc274072dfa02cd491efa79f3c8ae0a10774a0ece6c9c53b6a632cdef

LABEL org.opencontainers.image.title="The Search Database" \
      org.opencontainers.image.description="Non-root PostgreSQL and PostGIS runtime for The Search" \
      org.opencontainers.image.source="https://github.com/jonriber/the-search-surf" \
      org.opencontainers.image.licenses="Apache-2.0"

# Keep the immutable PostGIS base, but enforce a fixed-or-newer libexpat floor
# because upstream image rebuilds can lag published Alpine security packages.
# A lower bound avoids the stale exact-version failure mode while the generated
# SBOM and provenance record the version resolved during the build. The entry
# point uses gosu only when it starts as root; running as the existing postgres
# UID/GID removes that privilege transition and its otherwise unused Go binary
# from the runtime attack surface.
USER root
RUN apk add --no-cache --upgrade 'libexpat>=2.10.0-r0' && \
    rm /usr/local/bin/gosu
USER 70:70
