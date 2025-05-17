package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestPublishingInTransaction(t *testing.T) {
	DSN := "file:" + uuid.New().String() + "?mode=memory&journal_mode=WAL&busy_timeout=1000&secure_delete=true&foreign_keys=true&cache=shared"
	conn := newTestConnectionZombiezen(t, DSN)

	ctx, cancel := context.WithCancel(context.TODO()) // TODO: replace with t.Context() when Watermill bumps up to 1.24
	defer cancel()
	topic := "TestPublishingInTransaction"
	tg := wmsqlitezombiezen.TableNameGenerators{}.WithDefaultGeneratorsInsteadOfNils()
	if err := wmsqlitezombiezen.CreateTopicAndOffsetsTablesIfAbsent(
		conn,
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

	closer := sqlitex.Transaction(conn)

	pub0, err := wmsqlitezombiezen.NewPublisher(
		conn,
		wmsqlitezombiezen.PublisherOptions{
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
	if closer(&err); err != nil {
		t.Fatal("failed to commit the transaction:", err)
	}

	// rows, err := conn.Query("SELECT * FROM " + tg.Topic(topic))
	// if err != nil {
	// 	t.Fatal("unable to query rows:", err)
	// }
	// if err = rows.Err(); err != nil {
	// 	t.Fatal("rows query failed:", err)
	// }
	// count := 0
	// for rows.Next() {
	// 	count++
	// }
	// if count != 3 {
	// 	t.Fatal("expected 3 rows but got", count)
	// }
	// if err := rows.Close(); err != nil {
	// 	t.Fatal("unable to release rows:", err)
	// }

	sub, err := wmsqlitezombiezen.NewSubscriber(DSN, wmsqlitezombiezen.SubscriberOptions{
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
