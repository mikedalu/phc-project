# ═══════════════════════════════════════════════════════════════
#  PHC Records API — multi-stage build
#
#  Build context is `api/`, so Cloud Run must be deployed from there:
#      cd api && gcloud run deploy phc-api --source .
#
#  Stage 1 compiles a static binary; stage 2 ships only the binary.
#  Result: ~18 MB image with no shell, no package manager, and no libc
#  for an attacker to pivot through.
# ═══════════════════════════════════════════════════════════════

# ── Stage 1: build ─────────────────────────────────────────────
FROM golang:1.26-alpine AS build

WORKDIR /src

# Copy the module files FIRST and download dependencies as a separate layer.
# Docker caches this layer, so editing a .go file does not re-download the
# entire dependency graph on every build.
COPY go.mod go.sum ./
RUN go mod download

# Now copy the source. Any change here invalidates only the layers below.
COPY . .

# CGO_ENABLED=0 produces a fully static binary, which is what allows the
# distroless/static runtime image below (no musl, no glibc).
#
#   -s -w          strip the symbol table and DWARF data  (~30% smaller)
#   -X main.buildVersion  stamp the version so /health can report it
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
      -ldflags="-s -w -X main.buildVersion=${VERSION}" \
      -o /out/api ./cmd/api

# ── Stage 2: runtime ───────────────────────────────────────────
# static-debian12 has no shell, no package manager and no libc. If an
# attacker achieves RCE, there is nothing on the filesystem to use.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api

# Cloud Run injects PORT and health-checks that exact port. This default is
# only a fallback; main.go reads the variable (see 00-ARCHITECTURE.md §3.1).
ENV PORT=8080
EXPOSE 8080

# Run as the non-root user provided by the distroless image.
USER nonroot:nonroot

ENTRYPOINT ["/api"]
