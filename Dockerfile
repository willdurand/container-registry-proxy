FROM golang:1.24-alpine as builder

WORKDIR /usr/src/app

# Pre-copy/cache go.mod for pre-downloading dependencies and only redownloading
# them in subsequent builds if they change.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/src/app/app ./...

FROM alpine:3

COPY --from=builder /usr/src/app/app /usr/local/bin/app

EXPOSE 10000

CMD ["app"]
