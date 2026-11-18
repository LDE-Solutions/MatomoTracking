# Traefik image with this plugin built in, loaded as a local plugin (no plugin catalog needed).
# The static configuration must declare it, see "Setup instructions" in README.md.
#
#   docker build -t traefik-matomo .
#   docker build -t traefik-matomo --build-arg TRAEFIK_VERSION=3.7 .
ARG TRAEFIK_VERSION=3

# Collect the sources Traefik interprets at runtime, without tests, docs and examples.
FROM alpine:3.22 AS plugin
WORKDIR /plugin
COPY go.mod .traefik.yml LICENSE matomotracking.go ./
COPY internal ./internal
RUN find . -name '*_test.go' -delete

FROM traefik:${TRAEFIK_VERSION}
LABEL org.opencontainers.image.source="https://github.com/LDE-Solutions/MatomoTracking"
# The path must match the module path in go.mod and the moduleName in the static configuration.
COPY --from=plugin /plugin /plugins-local/src/github.com/DIE-Bonn/MatomoTracking
