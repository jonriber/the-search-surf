# syntax=docker/dockerfile:1.20

ARG NODE_IMAGE=node:24.14.1-alpine@sha256:8510330d3eb72c804231a834b1a8ebb55cb3796c3e4431297a24d246b8add4d5
ARG NGINX_IMAGE=nginxinc/nginx-unprivileged:stable-alpine@sha256:44e36330f74d4f3a1d4e222acca9e23b401fb87811a7597024502bb759c4dd49

FROM ${NODE_IMAGE} AS build

WORKDIR /src

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . ./
RUN npm run build

FROM ${NGINX_IMAGE}

# The pinned base still ships OpenSSL packages affected by CVE-2026-14456.
USER root
RUN apk add --no-cache --upgrade \
    libcrypto3=3.5.8-r0 \
    libssl3=3.5.8-r0

LABEL org.opencontainers.image.title="The Search PWA" \
      org.opencontainers.image.description="Installable client for explainable, personalized surf recommendations" \
      org.opencontainers.image.source="https://github.com/jonriber/the-search-surf" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --chown=nginx:nginx nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build --chown=nginx:nginx /src/dist /usr/share/nginx/html

USER nginx
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["wget", "--quiet", "--output-document=-", "http://127.0.0.1:8080/health/live"]
