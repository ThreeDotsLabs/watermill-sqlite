package tests

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

func TestPublishingInTransaction_modernc(t *testing.T) {
	db := newTestConnectionModernC(t, "file:"+uuid.New().String()+"?mode=memory&journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared")

	ctx, cancel := context.WithCancel(context.TODO()) // TODO: replace with t.Context() when Watermill bumps up to 1.24
	defer cancel()
	topic := "TestPublishingInTransaction"
	tg := wmsqlitemodernc.TableNameGenerators{}.WithDefaultGeneratorsInsteadOfNils()
	if err := wmsqlitemodernc.CreateTopicAndOffsetsTablesIfAbsent(
		ctx,
		db,
		tg.Topic(topic),
		tg.Offsets(topic),
	); err != nil {
		t.Fatal("unable to manually initialize tables:", err)
	}

	messagesToPublish := [...]*message.Message{
		message.NewMessage("0", []byte("payload0")),
		message.NewMessage("1", []byte("payload1")),
		message.NewMessage("2", []byte("payload2")),
	}

	tx0, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		t.Fatal(err)
	}

	pub0, err := wmsqlitemodernc.NewPublisher(
		tx0,
		wmsqlitemodernc.PublisherOptions{
			// ParentContext: t.Context(), // TODO: when Watermill upgrades to Golang 1.24
			TableNameGenerators: tg,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = pub0.Publish(topic, messagesToPublish[:2]...); err != nil {
		t.Fatal("cannot publish the messages:", err)
	}
	if err = pub0.Publish(topic, messagesToPublish[2]); err != nil {
		t.Fatal("cannot publish the messages:", err)
	}
	if err = tx0.Commit(); err != nil {
		t.Fatal("failed to commit the transaction:", err)
	}

	rows, err := db.Query("SELECT * FROM " + tg.Topic(topic))
	if err != nil {
		t.Fatal("unable to query rows:", err)
	}
	if err = rows.Err(); err != nil {
		t.Fatal("rows query failed:", err)
	}
	count := 0
	for rows.Next() {
		count++
	}
	if count != 3 {
		t.Fatal("expected 3 rows but got", count)
	}
	if err := rows.Close(); err != nil {
		t.Fatal("unable to release rows:", err)
	}

	sub, err := wmsqlitemodernc.NewSubscriber(db, wmsqlitemodernc.SubscriberOptions{
		PollInterval:        time.Millisecond * 20,
		LockTimeout:         time.Second,
		InitializeSchema:    true,
		TableNameGenerators: tg,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sub.Close(); err != nil {
			t.Fatal("unable to close subscriber", err)
		}
	})
	messagesFromSubscriber, err := sub.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case msg0 := <-messagesFromSubscriber:
		if msg0.UUID != messagesToPublish[0].UUID {
			t.Errorf("expected message with UUID %s but got %s", messagesToPublish[0].UUID, msg0.UUID)
		}
		msg0.Ack()
	case <-time.After(time.Second * 2):
		t.Fatal("timeout waiting for the first message")
	}

	select {
	case msg1 := <-messagesFromSubscriber:
		if msg1.UUID != messagesToPublish[1].UUID {
			t.Errorf("expected message with UUID %s but got %s", messagesToPublish[1].UUID, msg1.UUID)
		}
		msg1.Ack()
	case <-time.After(time.Second * 2):
		t.Fatal("timeout waiting for the second message")
	}

	select {
	case msg2 := <-messagesFromSubscriber:
		if msg2.UUID != messagesToPublish[2].UUID {
			t.Errorf("expected message with UUID %s but got %s", messagesToPublish[2].UUID, msg2.UUID)
		}
		msg2.Ack()
	case <-time.After(time.Second * 2):
		t.Fatal("timeout waiting for the third message")
	}
}

// TestConcurrentTransactions checks if messages are not lost when they are published within concurrent transactions.
// This was a bug in the implementation of the PostgreSQL adapter.
// Test imitates the TestNotMissingMessages test found here: https://github.com/ThreeDotsLabs/watermill-sql/blob/master/pkg/sql/pubsub_test.go
//
// See more: https://github.com/ThreeDotsLabs/watermill/issues/311
func TestConcurrentTransactions(t *testing.T) {
	t.Skip("messages are never lost and the test simply dead locks because there is only one connection")
	t.Run("concurrent transaction with :memory: connection", testConcurrentTransactions(":memory:?journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared"))
}

func testConcurrentTransactions(connectionDSN string) func(*testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		db := newTestConnectionModernC(t, connectionDSN)
		topic := "TestConcurrentTransactions"
		sub, err := wmsqlitemodernc.NewSubscriber(db, wmsqlitemodernc.SubscriberOptions{
			InitializeSchema: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := sub.Close(); err != nil {
				t.Fatal("unable to close subscriber", err)
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		messagesFromSubscriber, err := sub.Subscribe(ctx, topic)
		if err != nil {
			t.Fatal(err)
		}
		messagesReceived := make(chan *message.Message)
		go func(ctx context.Context) {
			defer close(messagesReceived)
			for {
				select {
				case msg, ok := <-messagesFromSubscriber:
					if !ok {
						return
					}
					msg.Ack()
					messagesReceived <- msg
				case <-time.After(time.Second * 3):
					return
				case <-ctx.Done():
					return
				}
			}
		}(ctx)

		tx0, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond * 10)

		tx1, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond * 10)

		txRollback, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond * 10)

		tx2, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond * 10)

		messagesToPublish := [...]*message.Message{
			message.NewMessage("0", []byte("payload0")),
			message.NewMessage("1", []byte("payload1")),
			message.NewMessage("2", []byte("payload2")),
		}

		pub0, err := wmsqlitemodernc.NewPublisher(
			tx0,
			wmsqlitemodernc.PublisherOptions{
				// ParentContext: t.Context(), // TODO: when Watermill upgrades to Golang 1.24
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err = pub0.Publish(topic, messagesToPublish[0]); err != nil {
			t.Fatal("cannot publish the first message:", err)
		}

		pub1, err := wmsqlitemodernc.NewPublisher(
			tx1,
			wmsqlitemodernc.PublisherOptions{
				// ParentContext: t.Context(), // TODO: when Watermill upgrades to Golang 1.24
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err = pub1.Publish(topic, messagesToPublish[1]); err != nil {
			t.Fatal("cannot publish the second message:", err)
		}

		pubRollback, err := wmsqlitemodernc.NewPublisher(
			txRollback,
			wmsqlitemodernc.PublisherOptions{
				// ParentContext: t.Context(), // TODO: when Watermill upgrades to Golang 1.24
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err = pubRollback.Publish(topic, message.NewMessage("rollback", []byte("rollback"))); err != nil {
			t.Fatal("cannot publish the roll-back message:", err)
		}

		pub2, err := wmsqlitemodernc.NewPublisher(
			tx2,
			wmsqlitemodernc.PublisherOptions{
				// ParentContext: t.Context(), // TODO: when Watermill upgrades to Golang 1.24
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err = pub2.Publish(topic, messagesToPublish[2]); err != nil {
			t.Fatal("cannot publish the third message:", err)
		}

		if err = tx2.Commit(); err != nil {
			t.Fatal("cannot commit the tx2 transaction:", err)
		}
		time.Sleep(time.Millisecond * 10)
		if err = txRollback.Rollback(); err != nil {
			t.Fatal("cannot commit the roll-back transaction:", err)
		}
		time.Sleep(time.Millisecond * 10)
		if err = tx1.Commit(); err != nil {
			t.Fatal("cannot commit the tx1 transaction:", err)
		}
		time.Sleep(time.Millisecond * 10)
		if err = tx0.Commit(); err != nil {
			t.Fatal("cannot commit the tx0 transaction:", err)
		}
		time.Sleep(time.Millisecond * 10)

		collectedUUIDs := make(map[string]struct{})
		for msg := range messagesReceived {
			collectedUUIDs[msg.UUID] = struct{}{}
		}
		if len(collectedUUIDs) != len(messagesToPublish) {
			t.Fatalf("expected to collect %d messages, got %d", len(messagesToPublish), len(collectedUUIDs))
		}
		for _, msg := range messagesToPublish {
			if _, ok := collectedUUIDs[msg.UUID]; !ok {
				t.Errorf("expected to collect message with UUID %s but it is missing", msg.UUID)
			}
		}
	}
}
