FROM golang:1.27-alpine AS build

WORKDIR /src

COPY services/api/go.mod ./go.mod
COPY services/api/go.sum ./go.sum
RUN go mod download

COPY services/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8080

USER nobody
ENTRYPOINT ["api"]
