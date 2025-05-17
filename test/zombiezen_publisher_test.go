package tests

import (
	"testing"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestPublisher(t *testing.T) {
	conn := newTestConnectionZombiezen(t, ":memory:")
	pub, err := wmsqlitezombiezen.NewPublisher(conn, wmsqlitezombiezen.PublisherOptions{
		InitializeSchema: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pub.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if err = pub.Publish(
		"test-topic",
		message.NewMessage(uuid.New().String(), []byte("test")),
		message.NewMessage(uuid.New().String(), []byte("test")),
	); err != nil {
		t.Fatal(err)
	}

	if err = sqlitex.ExecuteTransient(conn, "SELECT uuid FROM 'watermill_test-topic'", &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			t.Log("discovered message:", stmt.ColumnText(0))
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	// t.Fatal("implement")
}
