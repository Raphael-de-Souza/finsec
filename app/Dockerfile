# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 -> Forces Go to use 100% static binary to not rely on external libraries.  
# -trimpath -> Remove absolute paths from the binary construction system. More security. Do not expose internal paths 
# -ldflags="-s -w" -> -s: Omit Symbol Table. Symbol table maps the memory address and the functions name. Less detailed stacktrace  / -w: Omit DWARF will not be able to debug line to line but it will be considerable smaller
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \ 
    go build -trimpath -ldflags="-s -w" \
    -o /bin/api ./cmd/api

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
# Empty image. Works because the binary was compiled with CGO_ENABLED=0 
FROM scratch

# copy CA certificates for any outbound HTTPS calls
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /bin/api /api

EXPOSE 8080

ENTRYPOINT ["/api"]
