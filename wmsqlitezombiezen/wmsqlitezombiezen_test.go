package wmsqlitezombiezen

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/test"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"zombiezen.com/go/sqlite"
)

func newTestConnection(t *testing.T, connectionDSN string) *sqlite.Conn {
	conn, err := sqlite.OpenConn(connectionDSN)
	if err != nil {
		t.Fatal("unable to create test SQLite connetion", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatal("unable to close test SQLite connetion", err)
		}
	})
	// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
	// conn.SetInterrupt(t.Context().Done())
	return conn
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

		sub, err := NewSubscriber(connectionDSN,
			SubscriberOptions{
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

func TestFullConfirmityToModerncImplementation(t *testing.T) {
	// if !testing.Short() {
	// 	t.Skip("working on acceptance tests")
	// }
	t.Run("importedTestsFromModernc", func(t *testing.T) {
		// fixture := NewFileDB(t)
		fixture := NewEphemeralDB(t)
		t.Run("basic functionality", test.TestBasicSendRecieve(fixture))
		t.Run("one publisher three subscribers", test.TestOnePublisherThreeSubscribers(fixture, 1000))
		t.Run("perpetual locks", test.TestHungOperations(fixture))
	})

	t.Run("acceptanceTestsImportedFromModernc", func(t *testing.T) {
		if testing.Short() {
			t.Skip("acceptance tests take several minutes to complete for all file and memory bound transactions")
		}
		t.Run("file bound transactions", test.OfficialImplementationAcceptance(test.PubSubFixture(NewFileDB(t))))
		t.Run("memory bound transactions", test.OfficialImplementationAcceptance(test.PubSubFixture(NewEphemeralDB(t))))
	})
}
