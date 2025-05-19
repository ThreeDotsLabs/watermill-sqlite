package wmsqlitemodernc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

// PublisherOptions configure message publishing behavior.
type PublisherOptions struct {
	// TableNameGenerators is a set of functions that generate table names for topics and offsets.
	// Defaults to [TableNameGenerators.WithDefaultGeneratorsInsteadOfNils].
	TableNameGenerators TableNameGenerators

	// InitializeSchema enables initialization of schema database during publish.
	// Schema is initialized once per topic per publisher instance.
	// InitializeSchema is forbidden if using an ongoing transaction as database handle.
	// It could result in an implicit commit of the transaction by a CREATE TABLE statement.
	InitializeSchema bool

	// Logger reports message publishing errors and traces. Defaults value is [watermill.NopLogger].
	Logger watermill.LoggerAdapter
}

type publisher struct {
	InitializeSchema          bool
	TopicTableNameGenerator   TableNameGenerator
	OffsetsTableNameGenerator TableNameGenerator
	UUID                      string
	DB                        SQLiteConnection
	Logger                    watermill.LoggerAdapter

	mu          sync.Mutex
	closed      bool
	knownTopics map[string]struct{}
}

// NewPublisher creates a [message.Publisher] instance from a connection interface which could be
// a database handle or a transaction.
func NewPublisher(db SQLiteConnection, options PublisherOptions) (message.Publisher, error) {
	if db == nil {
		return nil, ErrDatabaseConnectionIsNil
	}
	if options.InitializeSchema && isTx(db) {
		return nil, ErrAttemptedTableInitializationWithinTransaction
	}
	if options.Logger == nil {
		options.Logger = defaultLogger
	}

	ID := uuid.New().String()
	tng := options.TableNameGenerators.WithDefaultGeneratorsInsteadOfNils()
	return &publisher{
		InitializeSchema:          options.InitializeSchema,
		UUID:                      ID,
		DB:                        db,
		TopicTableNameGenerator:   tng.Topic,
		OffsetsTableNameGenerator: tng.Offsets,
		Logger: options.Logger.With(watermill.LogFields{
			"publisher_id": ID,
		}),
		mu:          sync.Mutex{},
		knownTopics: make(map[string]struct{}),
	}, nil
}

// Publish pushes messages into a topic. Returns [ErrPublisherIsClosed] if the publisher is closed.
func (p *publisher) Publish(topic string, messages ...*message.Message) (err error) {
	if p.IsClosed() {
		return ErrPublisherIsClosed
	}
	if len(messages) == 0 {
		return nil
	}
	messagesTableName := p.TopicTableNameGenerator(topic)

	// Using the context of the first message
	// fails the official tests.TestMessageCtx acceptance
	// test. The test cancels the context before
	// publishing messages and expects the publishing to be successful.
	// The only way to do that is to use another context.
	//
	// ctx := messages[0].Context()
	ctx := context.Background()
	err = p.initializeSchema(ctx, topic, messagesTableName)
	if err != nil {
		return fmt.Errorf("unable to initialize schema for topic %q: %w", topic, err)
	}

	b := strings.Builder{}
	_, _ = b.WriteString("INSERT INTO '")
	_, _ = b.WriteString(messagesTableName)
	_, _ = b.WriteString("' (uuid, created_at, payload, metadata) VALUES ")

	values := make([]any, 0, len(messages)*4)
	for _, msg := range messages {
		metadata, err := json.Marshal(msg.Metadata)
		if err != nil {
			return fmt.Errorf("unable to encode message %q metadata to JSON: %w", msg.UUID, err)
		}
		values = append(values, msg.UUID, time.Now().Format(time.RFC3339), msg.Payload, metadata)
		b.WriteString(`(?,?,?,?),`)
	}

	query := strings.TrimRight(b.String(), ",")
	p.Logger.Trace("Inserting messages into SQLite table", watermill.LogFields{
		"query":      query,
		"query_args": values,
	})
	_, err = p.DB.ExecContext(ctx, query, values...)
	return err
}

func (p *publisher) initializeSchema(
	parent context.Context,
	topic string,
	messagesTableName string,
) (err error) {
	if !p.InitializeSchema {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, time.Second*60)
	defer cancel()
	p.mu.Lock()
	if _, ok := p.knownTopics[topic]; !ok {
		if err = createTopicAndOffsetsTablesIfAbsent(
			ctx,
			p.DB,
			messagesTableName,
			p.OffsetsTableNameGenerator(topic),
		); err != nil {
			p.mu.Unlock()
			return err
		}
		p.knownTopics[topic] = struct{}{}
	}
	p.mu.Unlock()
	return nil
}

func (p *publisher) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *publisher) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}

func (p *publisher) String() string {
	return "sqlite3-modernc-publisher-" + p.UUID
}

func isTx(db SQLiteConnection) bool {
	_, dbIsTx := db.(interface {
		Commit() error
		Rollback() error
	})
	return dbIsTx
}
