# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.27 AS build
WORKDIR /src
# pkgs is resolved at build time via the go.mod replace
# (github.com/rufftruffles/malice-plugins), so only this repo is copied.
COPY malice/ /src/malice/
WORKDIR /src/malice
RUN go mod download
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -buildvcs=false \
    -ldflags "-X main.version=${VERSION}" \
    -o /out/malice .

# ---- Runtime stage ----
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/malice /usr/local/bin/malice
ENV MALICE_ELASTICSEARCH_URL=http://elasticsearch:9200 \
    MALICE_ELASTICSEARCH_INDEX=malice
EXPOSE 3993
ENTRYPOINT ["malice"]
CMD ["serve", "--port", "3993"]
