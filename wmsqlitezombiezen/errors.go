package wmsqlitezombiezen

// Error represents common errors that might occur during the SQLite driver configuration or operations.
type Error uint8

const (
	// ErrUnknown indicates that operation failed due to an unknown reason.
	ErrUnknown Error = iota

	// ErrDatabaseConnectionIsNil indicates that configuration contained a nil database connection.
	ErrDatabaseConnectionIsNil

	// ErrPublisherIsClosed indicates that the publisher is closed and does not accept any more events.
	ErrPublisherIsClosed

	// ErrSubscriberIsClosed indicates that the subscriber is closed can no longer respond to events.
	ErrSubscriberIsClosed

	// ErrAttemptedTableInitializationWithinTransaction indicates that a database handle is a transaction
	// while trying to initialize SQLite tables. SQLite does not support table creation within transactions.
	// Attempting to create a table within a transaction can lead to data inconsistencies and errors.
	//
	// A transaction is probably used for single use event operations. Attempting to create a table
	// in such a scenario adds unnecessary overhead. Initialize the tables once when the application starts.
	ErrAttemptedTableInitializationWithinTransaction

	// ErrInvalidTopicName indicates that the topic name contains invalid characters.
	// Valid characters match the following regular expression pattern: `[^A-Za-z0-9\-\$\:\.\_]`.
	ErrInvalidTopicName

	// ErrConsumerGroupIsLocked indicates the failure to acquire a row lock because another consumer
	// in the same group has already acquired it. This a sentinel error for [subscription.NextBatch].
	// You should never see this error.
	ErrConsumerGroupIsLocked

	// ErrMoreRowStepsThanExpected indicates that an SQLite statement returned more result rows than expected.
	// This can only happen if there is a mistake in the SQLite query. Can occur if more than one row is returned
	// when one was expected or none were expected. You should never see this error.
	ErrMoreRowStepsThanExpected

	// ErrEmptyBatch is a sentinel error indicating the absence of new topic messages.
	// This error is used to cancel the database transaction and the row lock
	// when there are no new messages available. This prevents the subscriber
	// from writing to the database every poll interval when the topic is idle.
	ErrEmptyBatch
)

func (e Error) Error() string {
	switch e {
	case ErrDatabaseConnectionIsNil:
		return "SQLite database connection is nil"
	case ErrPublisherIsClosed:
		return "publisher is closed and does not accept any more events"
	case ErrSubscriberIsClosed:
		return "subscriber is closed and can no longer respond to events"
	case ErrAttemptedTableInitializationWithinTransaction:
		return "attempted table initialization with-in a transaction; either use a prior schema or do not combine a transaction with AutoInitializeSchema configuration option"
	case ErrInvalidTopicName:
		return "topic name must not contain characters matched by " + disallowedTopicCharacters.String()
	case ErrConsumerGroupIsLocked:
		return "consumer group is already locked by another consumer"
	case ErrMoreRowStepsThanExpected:
		return "more rows returned than expected"
	case ErrEmptyBatch:
		return "there are no new messages"
	default:
		return "unknown error"
	}
}

var errEmptyBatch error = ErrEmptyBatch // reference container for the error
