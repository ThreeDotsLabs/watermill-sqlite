package wmsqlitemodernc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

const (
	// DefaultMessageBatchSize is the default number of messages
	// for [SubscriberOptions] that a subscription
	// will collect when consuming messages from the database.
	DefaultMessageBatchSize = 100

	// DefaultSubscriberLockTimeout is the default duration of the row lock
	// setting for [SubscriberOptions]. Must be in full seconds.
	DefaultSubscriberLockTimeout = 5 * time.Second

	// DefaultAckDeadline is the default duration of the message acknowledgement deadline
	// setting for [SubscriberOptions].
	DefaultAckDeadline = 30 * time.Second

	// DefaultConsumerGroupName is the default subscription
	// consumer group name.
	DefaultConsumerGroupName = "default"
)

// ConsumerGroupMatcher associates a subscriber with a consumer
// group based on the subscription topic name.
type ConsumerGroupMatcher interface {
	// MatchTopic returns a consumer group name
	// for a given topic. This name must follow the same
	// naming conventions as the topic name.
	MatchTopic(topic string) (consumerGroupName string, err error)
}

// ConsumerGroupMatcherFunc is a convenience type that
// implements the [ConsumerGroupMatcher] interface.
type ConsumerGroupMatcherFunc func(topic string) (consumerGroupName string, err error)

// MatchTopic satisfies the [ConsumerGroupMatcher] interface.
func (f ConsumerGroupMatcherFunc) MatchTopic(topic string) (consumerGroupName string, err error) {
	return f(topic)
}

// NewStaticConsumerGroupMatcher creates a new [ConsumerGroupMatcher] that
//
//	returns the same consumer group name for any topic.
func NewStaticConsumerGroupMatcher(consumerGroupName string) ConsumerGroupMatcher {
	return ConsumerGroupMatcherFunc(func(topic string) (string, error) {
		return consumerGroupName, nil
	})
}

var defaultConsumerGroupMatcher ConsumerGroupMatcher = NewStaticConsumerGroupMatcher(DefaultConsumerGroupName)

// SubscriberOptions defines options for creating a subscriber. Every selection has a reasonable default value.
type SubscriberOptions struct {
	// ConsumerGroupMatcher differentiates message consumers within the same topic.
	// Messages are processed in batches.
	// Therefore, another subscriber with the same consumer group name may only obtain
	// messages whenever it is able to acquire the row lock.
	// Default value is a static consumer group matcher that
	// always returns [DefaultConsumerGroupName].
	ConsumerGroupMatcher ConsumerGroupMatcher

	// BatchSize is the number of messages to read in a single batch.
	// Default value is [DefaultMessageBatchSize].
	BatchSize int

	// TableNameGenerators is a set of functions that generate table names for topics and offsets.
	// Default value is [TableNameGenerators.WithDefaultGeneratorsInsteadOfNils].
	TableNameGenerators TableNameGenerators

	// PollInterval is the interval to wait between subsequent SELECT queries, if no more messages were found in the database (Prefer using the BackoffManager instead).
	// Must be non-negative. Defaults to one second.
	PollInterval time.Duration

	// LockTimeout is the maximum duration of the row lock. If the subscription
	// is unable to extend the lock before this time out ends, the lock will expire.
	// Then, another subscriber in the same consumer group name may
	// acquire the lock and continue processing messages.
	//
	// Duration must not be less than one second, because seconds are added
	// to the SQLite `unixepoch` function, rounded to the nearest second.
	// A zero duration would create a lock that expires immediately.
	// There is no reason to set higher precision fractional duration,
	// because the lock timeout will rarely ever trigger in a healthy system.
	// Normally, the row lock is set to zero after each batch of messages is processed. LockTimeout might occur if a consuming node shuts down unexpectedly,
	// before it is able to complete processing a batch of messages. Only
	// in such rare cases the time out matters. And, it is better to set it
	// to a higher value in order to avoid unnecessary batch re-processing.
	// Therefore, a value lower than one second is impractical.
	//
	// Default value is [DefaultLockTimeout].
	LockTimeout time.Duration

	// AckDeadline is the time to wait for acking a message.
	// If message is not acked within this time, it will be nacked and re-delivered.
	//
	// When messages are read in bulk, this time is calculated for each message separately.
	//
	// If you want to disable the acknowledgement deadline, set it to 0.
	// Warning: when acknowledgement deadline is disabled, messages may block and
	// prevent the subscriber from accepting new messages.
	//
	// Must be non-negative. Default value is [DefaultAckDeadline].
	AckDeadline *time.Duration

	// InitializeSchema option enables initializing schema on making a subscription.
	InitializeSchema bool

	// Logger reports message consumption errors and traces. Defaults to [watermill.NopLogger].
	Logger watermill.LoggerAdapter
}

type subscriber struct {
	DB                        SQLiteDatabase
	UUID                      string
	PollInterval              time.Duration
	LockTimeoutInSeconds      int
	InitializeSchema          bool
	ConsumerGroupMatcher      ConsumerGroupMatcher
	BatchSize                 int
	NackChannel               func() <-chan time.Time
	Closed                    chan struct{}
	TopicTableNameGenerator   TableNameGenerator
	OffsetsTableNameGenerator TableNameGenerator
	Logger                    watermill.LoggerAdapter
	Subscriptions             *sync.WaitGroup
}

// NewSubscriber creates a new subscriber with the given options.
func NewSubscriber(db SQLiteDatabase, options SubscriberOptions) (message.Subscriber, error) {
	if db == nil {
		return nil, ErrDatabaseConnectionIsNil
	}
	if options.ConsumerGroupMatcher == nil {
		options.ConsumerGroupMatcher = defaultConsumerGroupMatcher
	}
	if options.BatchSize < 0 {
		return nil, errors.New("BatchSize must be greater than 0")
	}
	if options.BatchSize > 1_000_000 {
		return nil, errors.New("BatchSize must be less than a million")
	}
	if options.PollInterval != 0 && options.PollInterval < time.Millisecond {
		return nil, errors.New("PollInterval must be greater than one millisecond")
	}
	if options.PollInterval > time.Hour*24*7 {
		return nil, errors.New("PollInterval must be less than a week")
	}
	if options.LockTimeout < time.Second {
		if options.LockTimeout == 0 {
			options.LockTimeout = DefaultSubscriberLockTimeout
		} else {
			return nil, errors.New("LockTimeout must be greater than one second")
		}
	}

	nackChannel := func() <-chan time.Time {
		// by default, Nack messages if they take longer than 30 seconds to process
		return time.After(DefaultAckDeadline)
	}
	if options.AckDeadline != nil {
		deadline := *options.AckDeadline
		if deadline < 0 {
			return nil, errors.New("AckDeadline must be above 0")
		}
		if deadline == 0 {
			nackChannel = func() <-chan time.Time {
				// infinite: always blocked
				return nil
			}
		} else {
			nackChannel = func() <-chan time.Time {
				return time.After(deadline)
			}
		}
	}

	ID := uuid.New().String()
	tng := options.TableNameGenerators.WithDefaultGeneratorsInsteadOfNils()
	return &subscriber{
		DB:                        db,
		UUID:                      ID,
		PollInterval:              cmpOrTODO(options.PollInterval, time.Second),
		LockTimeoutInSeconds:      int(math.Round(options.LockTimeout.Seconds())),
		InitializeSchema:          options.InitializeSchema,
		ConsumerGroupMatcher:      options.ConsumerGroupMatcher,
		BatchSize:                 cmpOrTODO(options.BatchSize, DefaultMessageBatchSize),
		NackChannel:               nackChannel,
		Closed:                    make(chan struct{}),
		TopicTableNameGenerator:   tng.Topic,
		OffsetsTableNameGenerator: tng.Offsets,
		Logger: cmpOrTODO[watermill.LoggerAdapter](
			options.Logger,
			defaultLogger,
		).With(watermill.LogFields{
			"subscriber_id": ID,
		}),
		Subscriptions: &sync.WaitGroup{},
	}, nil
}

// Subscribe streams messages from the topic. Satisfies [watermill.Subscriber] interface.
// Returns [ErrSubscriberIsClosed] if the subscriber is closed.
func (s *subscriber) Subscribe(ctx context.Context, topic string) (c <-chan *message.Message, err error) {
	if s.IsClosed() {
		return nil, ErrSubscriberIsClosed
	}

	consumerGroup, err := s.ConsumerGroupMatcher.MatchTopic(topic)
	if err != nil {
		return nil, fmt.Errorf("unable to match topic to a consumer group: %w", err)
	}
	if err = validateTopicName(consumerGroup); err != nil {
		return nil, fmt.Errorf("consumer group name must follow the same validation rules as topic names: %w", err)
	}

	messagesTableName := s.TopicTableNameGenerator(topic)
	offsetsTableName := s.OffsetsTableNameGenerator(topic)
	if s.InitializeSchema {
		if err = CreateTopicAndOffsetsTablesIfAbsent(
			ctx,
			s.DB,
			messagesTableName,
			offsetsTableName,
		); err != nil {
			return nil, err
		}
	}

	_, err = s.DB.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO '%s' (consumer_group, offset_acked, locked_until)
		VALUES ("%s", 0, 0)
		ON CONFLICT(consumer_group) DO NOTHING;
	`, offsetsTableName, consumerGroup))
	if err != nil {
		return nil, err
	}

	sub := &subscription{
		DB:           s.DB,
		pollTicker:   time.NewTicker(s.PollInterval),
		lockDuration: time.Second*time.Duration(s.LockTimeoutInSeconds) - (time.Millisecond * 300), // less than the lock timeout
		nackChannel:  s.NackChannel,

		sqlLockConsumerGroup: fmt.Sprintf(
			`UPDATE '%s' SET locked_until=(unixepoch()+%d) WHERE consumer_group="%s" AND locked_until < unixepoch() RETURNING offset_acked`,
			offsetsTableName,
			s.LockTimeoutInSeconds,
			consumerGroup,
		),
		sqlExtendLock: fmt.Sprintf(
			`UPDATE '%s' SET locked_until=(unixepoch()+%d), offset_acked=? WHERE consumer_group="%s" AND offset_acked=? AND locked_until>=unixepoch() RETURNING COALESCE(locked_until, 0)`,
			offsetsTableName,
			s.LockTimeoutInSeconds,
			consumerGroup,
		),
		sqlNextMessageBatch: fmt.Sprintf(`
			SELECT "offset", uuid, payload, metadata
			FROM '%s'
			WHERE "offset">? ORDER BY offset LIMIT %d;
		`, messagesTableName, s.BatchSize),
		sqlAcknowledgeMessages: fmt.Sprintf(`
			UPDATE '%s' SET offset_acked=?, locked_until=0 WHERE consumer_group="%s" AND offset_acked = ?;
		`, offsetsTableName, consumerGroup),
		destination: make(chan *message.Message),
		logger: s.Logger.With(
			watermill.LogFields{
				"topic":          topic,
				"consumer_group": consumerGroup,
			},
		),
	}
	sub.lockTicker = time.NewTicker(sub.lockDuration)

	ctx, cancel := context.WithCancel(ctx)
	go func(done <-chan struct{}) {
		<-done
		cancel()
	}(s.Closed)

	s.Subscriptions.Add(1)
	go func(ctx context.Context) {
		defer s.Subscriptions.Done()
		sub.Run(ctx)
		// <-time.After(time.Second) // give a chance for mid-air transaction to commit
		close(sub.destination)
		cancel()
	}(ctx)

	return sub.destination, nil
}

// IsClosed returns true if the subscriber is closed.
func (s *subscriber) IsClosed() bool {
	select {
	case <-s.Closed:
		return true
	default:
		return false
	}
}

// Close terminates the subscriber and all its associated resources. Returns when everything is closed.
func (s *subscriber) Close() error {
	if !s.IsClosed() {
		close(s.Closed)
		s.Subscriptions.Wait()
	}
	return nil
}

func (s *subscriber) String() string {
	return "sqlite3-modernc-subscriber-" + s.UUID
}
