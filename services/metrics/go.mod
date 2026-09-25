module github.com/KRUTONIK/web-service-monitoring/services/metrics

go 1.27.0

require (
	github.com/KRUTONIK/web-service-monitoring/contracts v0.0.0
	google.golang.org/grpc v1.75.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
)

replace github.com/KRUTONIK/web-service-monitoring/contracts => ../../contracts
