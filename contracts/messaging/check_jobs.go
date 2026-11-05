package messaging

const (
	CheckJobsQueue          = "checker.jobs"
	CheckRetryExchange      = "checker.retry"
	CheckRetryQueue         = "checker.jobs.retry"
	CheckJobsDLQ            = "checker.jobs.dlq"
	CheckRetryRoute         = "retry"
	IncidentEventsQueue     = "incidents.events"
	NotificationEventsQueue = "notifications.events"
	NotificationRetryQueue  = "notifications.events.retry"
	NotificationDLQ         = "notifications.events.dlq"
	MaxCheckJobAttempts     = 3
)
