FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/shop ./cmd/shop

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shop /shop
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/shop", "-healthcheck"]
ENTRYPOINT ["/shop"]
