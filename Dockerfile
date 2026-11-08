# The build runs the tests: if they fail, there is no image to send emails with.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go vet ./... && go test ./... && CGO_ENABLED=0 go build -trimpath -o /santa .

# Just the binary and the CA certificates it needs to talk TLS to the mail server.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /santa /santa
WORKDIR /data
ENTRYPOINT ["/santa"]
