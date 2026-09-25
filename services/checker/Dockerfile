FROM golang:1.27-alpine AS build

WORKDIR /src

COPY contracts/ ./contracts/
COPY services/checker/ ./services/checker/
WORKDIR /src/services/checker
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/checker ./cmd/checker

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/checker /usr/local/bin/checker

USER nobody
ENTRYPOINT ["checker"]
