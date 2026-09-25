FROM golang:1.27-alpine AS build

WORKDIR /src

COPY contracts/ ./contracts/
COPY services/api/ ./services/api/
WORKDIR /src/services/api
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api

EXPOSE 8080
EXPOSE 9091

USER nobody
ENTRYPOINT ["api"]
