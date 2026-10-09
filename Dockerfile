FROM golang:1.25.0-alpine AS build

RUN apk update && apk add --no-cache git build-base libjpeg-turbo-dev libwebp-dev

WORKDIR /build

# Copiar apenas arquivos de dependências primeiro para cachear o download
COPY go.mod go.sum ./

# A dependência whatsmeow é fixada no go.mod, incluindo o patch controlado do fork.
RUN go mod download

# Copiar o restante do código
COPY . .

ARG VERSION
RUN release_version="$(cat VERSION)" && \
    test -n "$release_version" && \
    test "${VERSION:-$release_version}" = "$release_version" && \
    CGO_ENABLED=1 go build -mod=readonly -ldflags "-X main.version=${release_version}" -o server ./cmd/evolution-go

FROM alpine:3.19.1 AS final

# poppler-utils provides pdftoppm, used to rasterize PDF page 1 for /send/media document thumbnails
RUN apk update && apk add --no-cache tzdata ffmpeg libjpeg-turbo libwebp poppler-utils

WORKDIR /app

COPY --from=build /build/server .
COPY --from=build /build/manager/dist ./manager/dist
COPY --from=build /build/web ./web
COPY --from=build /build/VERSION ./VERSION

ENV TZ=America/Sao_Paulo

ENTRYPOINT ["/app/server"]
