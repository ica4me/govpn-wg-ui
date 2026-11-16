FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ARG VERSION=v1.5.0
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X govpn-wg-ui/internal/console.Version=${VERSION}" -o /ui ./cmd/ui
FROM scratch
COPY --from=build /ui /ui
USER 65532:65532
ENV GOVPN_UI_LISTEN=127.0.0.1:8090
ENTRYPOINT ["/ui"]
