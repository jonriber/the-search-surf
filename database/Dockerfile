FROM postgis/postgis:18-3.6-alpine@sha256:c53a483fc274072dfa02cd491efa79f3c8ae0a10774a0ece6c9c53b6a632cdef

LABEL org.opencontainers.image.title="The Search Database" \
      org.opencontainers.image.description="Non-root PostgreSQL and PostGIS runtime for The Search" \
      org.opencontainers.image.source="https://github.com/jonriber/the-search-surf" \
      org.opencontainers.image.licenses="Apache-2.0"

# The pinned base includes the fixed libexpat and libuuid releases that were
# previously installed from Alpine's mutable package repositories. Keeping the
# fixes inside the immutable base digest avoids stale exact apk pins breaking
# builds when repository indexes advance. The entry point uses gosu only when it
# starts as root; running as the existing postgres UID/GID removes that
# privilege transition and its otherwise unused Go binary from the runtime
# attack surface.
USER root
RUN rm /usr/local/bin/gosu
USER 70:70
