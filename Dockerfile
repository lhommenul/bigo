FROM golang:1.26-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /paperless-smtp-gateway .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /paperless-smtp-gateway /paperless-smtp-gateway
EXPOSE 25
ENTRYPOINT ["/paperless-smtp-gateway"]
