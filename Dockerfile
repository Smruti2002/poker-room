# Builds one small image holding the Go server and the built web app.
#   docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t poker-room .
#   docker run -p 8080:8080 poker-room

# --- Web app -------------------------------------------------------------
FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Server --------------------------------------------------------------
FROM golang:1.27.3-alpine AS server
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/server ./cmd/server

# --- Runtime -------------------------------------------------------------
# Distroless: no shell or package manager, just the binary and CA certs.
# It runs as root so it can write to the mounted volume (Fly volumes are
# root-owned); nothing else in the image is writable or worth attacking.
FROM gcr.io/distroless/static-debian12
COPY --from=server /out/server /server
COPY --from=web /src/web/dist /web
ENV PORT=8080 \
    STATIC_DIR=/web
EXPOSE 8080
ENTRYPOINT ["/server"]
