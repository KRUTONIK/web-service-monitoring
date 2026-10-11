FROM golang:1.25-alpine AS build

WORKDIR /src
COPY tests/http-fixture/main.go ./main.go
RUN CGO_ENABLED=0 go build -o /http-fixture ./main.go

FROM alpine:3.22
COPY --from=build /http-fixture /http-fixture
EXPOSE 8080
ENTRYPOINT ["/http-fixture"]
