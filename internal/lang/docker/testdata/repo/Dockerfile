# syntax=docker/dockerfile:1.7
# escape=\
ARG BASE=node:20-alpine
ARG REGISTRY
ARG GO_VERSION=1.22
ARG TAG

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS Build
RUN --mount=type=cache,target=/root/.cache \
    # a comment inside a continued instruction
    --mount=type=bind,from=alpine:3.20,source=/etc/ssl,target=/ssl \
    go build ./...

from ${BASE} as web
COPY --from=build /out /out
COPY --from=0 /go/bin/app /app
COPY --from=nginx:1.27@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef /etc/nginx /etc/nginx
RUN <<EOT
FROM not-an-instruction:latest
EOT

FROM ${REGISTRY}/tools:1 AS tools
FROM python:${TAG}
FROM docker.io/library/debian:bookworm-slim \
  AS runtime
FROM scratch
COPY --from=Runtime / /
ONBUILD COPY --from=busybox:1.36 /bin/sh /bin/sh
FROM web
