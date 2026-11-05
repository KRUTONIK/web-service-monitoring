FROM golang:1.27-alpine AS build
WORKDIR /src
COPY contracts/ ./contracts/
COPY services/notification/ ./services/notification/
WORKDIR /src/services/notification
RUN go mod download && CGO_ENABLED=0 GOOS=linux go build -o /out/notification .
FROM alpine:3.22
COPY --from=build /out/notification /usr/local/bin/notification
USER nobody
ENTRYPOINT ["notification"]
