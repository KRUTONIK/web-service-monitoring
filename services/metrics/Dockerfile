FROM golang:1.27-alpine AS build

WORKDIR /src

COPY contracts/ ./contracts/
COPY services/metrics/ ./services/metrics/
WORKDIR /src/services/metrics
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/metrics ./cmd/metrics

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/metrics /usr/local/bin/metrics

EXPOSE 9092

USER nobody
ENTRYPOINT ["metrics"]
