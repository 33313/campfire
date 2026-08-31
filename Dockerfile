# One binary, one container image (SPEC.md §13). The image carries the server,
# the web client and the fonts; the only thing to mount is the data directory.

# The client is compiled by Vite and embedded by the Go build, so it is built
# first and copied into place.
FROM node:22-alpine AS client
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY internal/web ./internal/web
COPY vite.config.js ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=client /src/internal/web/app ./internal/web/app
# CGO off: the SQLite driver is pure Go, so the binary is static and the
# runtime image needs nothing but a certificate bundle.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/campfire .

FROM alpine:3.20
RUN adduser -D -u 10001 campfire && mkdir -p /data && chown campfire /data
COPY --from=build /out/campfire /usr/local/bin/campfire
USER campfire
VOLUME /data
ENV XDG_DATA_HOME=/data
EXPOSE 8080

# The server binds to loopback unless told otherwise, which is right for a
# laptop and wrong inside a container, so the image configures the bind
# explicitly. Put a proxy in front of this; the README's "Behind a proxy"
# section has the buffering and timeout requirements the event stream has.
RUN printf '%s\n' \
    '{' \
    '  "addr": "0.0.0.0:8080",' \
    '  "database": "/data/campfire/campfire.db",' \
    '  "snapshots": {' \
    '    "enabled": true,' \
    '    "dir": "/data/campfire/snapshots",' \
    '    "interval": "6h",' \
    '    "keep": 14,' \
    '    "shipTo": ""' \
    '  },' \
    '  "mcp": { "enabled": false }' \
    '}' > /home/campfire/campfire.json

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/instance || exit 1

ENTRYPOINT ["campfire", "-config", "/home/campfire/campfire.json"]
