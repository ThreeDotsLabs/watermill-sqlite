package tests

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func newTestConnectionModernC(t *testing.T, connectionDSN string) *sql.DB {
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

func NewPubSubFixtureModernC(connectionDSN string) PubSubFixture {
	return func(t *testing.T, consumerGroup string) (message.Publisher, message.Subscriber) {
		publisherDB := newTestConnectionModernC(t, connectionDSN)

		pub, err := wmsqlitemodernc.NewPublisher(
			publisherDB,
			wmsqlitemodernc.PublisherOptions{
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

		subscriberDB := newTestConnectionModernC(t, connectionDSN)
		sub, err := wmsqlitemodernc.NewSubscriber(subscriberDB, wmsqlitemodernc.SubscriberOptions{
			PollInterval:         time.Millisecond * 20,
			ConsumerGroupMatcher: wmsqlitemodernc.NewStaticConsumerGroupMatcher(consumerGroup),
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

func NewEphemeralDBModernC(t *testing.T) PubSubFixture {
	return NewPubSubFixtureModernC("file:" + uuid.New().String() + "?mode=memory&journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared")
}

func NewFileDBModernC(t *testing.T) PubSubFixture {
	file := filepath.Join(t.TempDir(), uuid.New().String()+".sqlite3")
	t.Cleanup(func() {
		if err := os.Remove(file); err != nil {
			t.Fatal("unable to remove test SQLite database file", err)
		}
	})
	// &_txlock=exclusive
	return NewPubSubFixtureModernC("file:" + file + "?journal_mode=WAL&busy_timeout=5000&secure_delete=true&foreign_keys=true&cache=shared")
}

func TestPubSub_modernc(t *testing.T) {
	// if !testing.Short() {
	// 	t.Skip("working on acceptance tests")
	// }
	inMemory := NewEphemeralDBModernC(t)
	t.Run("basic functionality", TestBasicSendRecieve(inMemory))
	t.Run("one publisher three subscribers", TestOnePublisherThreeSubscribers(inMemory, 1000))
	t.Run("perpetual locks", TestHungOperations(inMemory))
}

func TestOfficialImplementationAcceptance_modern_c(t *testing.T) {
	if testing.Short() {
		t.Skip("acceptance tests take several minutes to complete for all file and memory bound transactions")
	}
	t.Run("file bound transactions", OfficialImplementationAcceptance(NewFileDBModernC(t)))
	t.Run("memory bound transactions", OfficialImplementationAcceptance(NewEphemeralDBModernC(t)))
}

func BenchmarkAll_modern_c(b *testing.B) {
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

	b.Run("go channel publishing", NewPublishingBenchmark(fastest))
	b.Run("go channel subscription", NewSubscriptionBenchmark(fastest))

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

	pub, err := wmsqlitemodernc.NewPublisher(db, wmsqlitemodernc.PublisherOptions{
		InitializeSchema: true,
	})
	if err != nil {
		b.Fatal("unable to create test publisher", err)
	}
	sub, err := wmsqlitemodernc.NewSubscriber(db, wmsqlitemodernc.SubscriberOptions{
		BatchSize:    700,
		PollInterval: time.Millisecond * 10,
	})
	if err != nil {
		b.Fatal("unable to create test subscriber", err)
	}
	b.Run("SQLite publishing to memory", NewPublishingBenchmark(pub))
	b.Run("SQLite subscription from memory", NewSubscriptionBenchmark(sub))
}
