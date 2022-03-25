FROM golang:1.14.14-buster AS builder

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download -x
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cheesegull

FROM alpine:latest
WORKDIR /app
COPY --from=builder /src/cheesegull /bin/
RUN chmod +x /bin/cheesegull
VOLUME [ "/app" ]
CMD ["/bin/cheesegull"]