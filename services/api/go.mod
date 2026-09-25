module github.com/KRUTONIK/web-service-monitoring/services/api

go 1.27.0

require (
	github.com/KRUTONIK/web-service-monitoring/contracts v0.0.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/rabbitmq/amqp091-go v1.15.0
	google.golang.org/grpc v1.75.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
)

replace github.com/KRUTONIK/web-service-monitoring/contracts => ../../contracts
