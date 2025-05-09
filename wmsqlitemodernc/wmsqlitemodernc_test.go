package wmsqlitemodernc

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/test"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func newTestConnection(t *testing.T, connectionDSN string) *sql.DB {
	db, err := sql.Open("sqlite", connectionDSN)
	if err != nil {
		t.Fatal("unable to create test SQLite connetion", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatal("unable to close test SQLite connetion", err)
		}
	})
	return db
}

func NewPubSubFixture(connectionDSN string) test.PubSubFixture {
	return func(t *testing.T, consumerGroup string) (message.Publisher, message.Subscriber) {
		publisherDB := newTestConnection(t, connectionDSN)

		pub, err := NewPublisher(
			publisherDB,
			PublisherOptions{
				InitializeSchema: true,
			})
		if err != nil {
			t.Fatal("unable to initialize publisher:", err)
		}
		t.Cleanup(func() {
			if err := pub.Close(); err != nil {
				t.Fatal(err)
			}
		})

		subscriberDB := newTestConnection(t, connectionDSN)
		sub, err := NewSubscriber(subscriberDB, SubscriberOptions{
			PollInterval:         time.Millisecond * 20,
			ConsumerGroupMatcher: NewStaticConsumerGroupMatcher(consumerGroup),
			InitializeSchema:     true,
		})
		if err != nil {
			t.Fatal("unable to initialize subscriber:", err)
		}
		t.Cleanup(func() {
			if err := sub.Close(); err != nil {
				t.Fatal(err)
			}
		})

		return pub, sub
	}
}

func NewEphemeralDB(t *testing.T) test.PubSubFixture {
	return NewPubSubFixture("file:" + uuid.New().String() + "?mode=memory&journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared")
}

func NewFileDB(t *testing.T) test.PubSubFixture {
	file := filepath.Join(t.TempDir(), uuid.New().String()+".sqlite3")
	t.Cleanup(func() {
		if err := os.Remove(file); err != nil {
			t.Fatal("unable to remove test SQLite database file", err)
		}
	})
	// &_txlock=exclusive
	return NewPubSubFixture("file:" + file + "?journal_mode=WAL&busy_timeout=5000&secure_delete=true&foreign_keys=true&cache=shared")
}

func TestPubSub(t *testing.T) {
	// if !testing.Short() {
	// 	t.Skip("working on acceptance tests")
	// }
	inMemory := NewEphemeralDB(t)
	t.Run("basic functionality", test.TestBasicSendRecieve(inMemory))
	t.Run("one publisher three subscribers", test.TestOnePublisherThreeSubscribers(inMemory, 1000))
	t.Run("perpetual locks", test.TestHungOperations(inMemory))
}

func TestOfficialImplementationAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("acceptance tests take several minutes to complete for all file and memory bound transactions")
	}
	t.Run("file bound transactions", test.OfficialImplementationAcceptance(NewFileDB(t)))
	t.Run("memory bound transactions", test.OfficialImplementationAcceptance(NewEphemeralDB(t)))
}

func BenchmarkAll(b *testing.B) {
	fastest := gochannel.NewGoChannel(gochannel.Config{
		// Output channel buffer size.
		// OutputChannelBuffer int64

		// If persistent is set to true, when subscriber subscribes to the topic,
		// it will receive all previously produced messages.
		//
		// All messages are persisted to the memory (simple slice),
		// so be aware that with large amount of messages you can go out of the memory.
		Persistent: true,

		// When true, Publish will block until subscriber Ack's the message.
		// If there are no subscribers, Publish will not block (also when Persistent is true).
		BlockPublishUntilSubscriberAck: false,
	}, nil)

	b.Run("go channel publishing", test.NewPublishingBenchmark(fastest))
	b.Run("go channel subscription", test.NewSubscriptionBenchmark(fastest))

	db, err := sql.Open("sqlite", "file:"+uuid.New().String()+"?mode=memory&journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared")
	if err != nil {
		b.Fatal("unable to create test SQLite connetion", err)
	}
	db.SetMaxOpenConns(1)
	b.Cleanup(func() {
		if err := db.Close(); err != nil {
			b.Fatal("unable to close test SQLite connetion", err)
		}
	})

	pub, err := NewPublisher(db, PublisherOptions{
		InitializeSchema: true,
	})
	if err != nil {
		b.Fatal("unable to create test publisher", err)
	}
	sub, err := NewSubscriber(db, SubscriberOptions{
		BatchSize:    700,
		PollInterval: time.Millisecond * 10,
	})
	if err != nil {
		b.Fatal("unable to create test subscriber", err)
	}
	b.Run("SQLite publishing to memory", test.NewPublishingBenchmark(pub))
	b.Run("SQLite subscription from memory", test.NewSubscriptionBenchmark(sub))
}
