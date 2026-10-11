package messaging

const (
	CheckJobsQueue      = "checker.jobs"
	CheckRetryExchange  = "checker.retry"
	CheckRetryQueue     = "checker.jobs.retry"
	CheckJobsDLQ        = "checker.jobs.dlq"
	CheckRetryRoute     = "retry"
	MaxCheckJobAttempts = 3
)
