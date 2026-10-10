FROM golang:1.27-alpine AS build

WORKDIR /src

COPY contracts/ ./contracts/
COPY services/scheduler/ ./services/scheduler/
WORKDIR /src/services/scheduler
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/scheduler ./cmd/scheduler

FROM alpine:3.22

RUN apk add --no-cache ca-certificates
COPY --from=build /out/scheduler /usr/local/bin/scheduler

USER nobody
ENTRYPOINT ["scheduler"]
