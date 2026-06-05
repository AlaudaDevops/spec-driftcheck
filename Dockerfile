FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /driftcheck .

FROM alpine:3.21
RUN apk add --no-cache git ca-certificates
COPY --from=build /driftcheck /usr/local/bin/driftcheck
ENTRYPOINT ["driftcheck"]
