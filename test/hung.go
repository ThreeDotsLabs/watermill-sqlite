package tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
)

func TestHungOperations(setup PubSubFixture) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		pub, sub := setup(t, "hungTests")

		t.Run("busy destination channel releases row lock", hungDestinationChannel(pub, sub))
		// t.Run("long ack does not lose row lock", hungLongAck(pub, sub))
		t.Run("nack does not cause an inifite loop", hungNackTest(pub, sub))
	}
}

func hungDestinationChannel(pub message.Publisher, sub message.Subscriber) func(t *testing.T) {
	topic := "hung-destination-channel-test"
	return func(t *testing.T) {
		t.Parallel()
		// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		err := pub.Publish(
			topic,
			message.NewMessage("first", []byte("payloadFirst")),
			message.NewMessage("second", []byte("payloadSecond")),
		)
		if err != nil {
			t.Fatal("failed to publish messages", err)
		}

		msgs1, err := sub.Subscribe(ctx, topic)
		if err != nil {
			t.Fatal(err)
		}

		<-time.After(time.Second)
		// first subscriber locked the row
		// add the second subscriber
		_, err = sub.Subscribe(ctx, topic)
		if err != nil {
			t.Fatal(err)
		}

		// TODO: this should use a constant lock duration
		// and this test must be turned into a sync test
		// to avoid waiting for the whole duration
		<-time.After(time.Second * 6)
		// first subscriber loses lock after grace period
		// but keeps the first message over its output channel
		// which will cause the second subscriber to receive
		// the same message and then process it twice

		var msg *message.Message
		select {
		case msg = <-msgs1:
		default:
		}

		if msg != nil && msg.UUID == "first" {
			t.Fatal("the same message was delivered twice", msg.UUID)
		}
	}
}

func hungLongAck(pub message.Publisher, sub message.Subscriber) func(t *testing.T) {
	topic := "hung-long-ack-test"
	return func(t *testing.T) {
		t.Parallel()
		// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		err := pub.Publish(
			topic,
			message.NewMessage("first", []byte("payloadFirst")),
			message.NewMessage("second", []byte("payloadSecond")),
		)
		if err != nil {
			t.Fatal("failed to publish messages", err)
		}

		msgs1, err := sub.Subscribe(ctx, topic)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgs1:
				t.Log("got message:", msg.UUID)
				// accept every message but never acknowledge it
			}
		}()

		<-time.After(time.Second)
		// first subscriber locked the row
		// add the second subscriber
		msgs2, err := sub.Subscribe(ctx, topic)
		if err != nil {
			t.Fatal(err)
		}

		/*
			If the lock was not extended, the second subscriber will obtain
			the lock and consume the first message. This causes a duplication.
		*/
		select {
		case msg := <-msgs2:
			t.Fatal("the second subscriber obtained the lock on message:", msg.UUID)
		case <-time.After(time.Second):
			// expected behavior
		}
	}
}

func hungNackTest(pub message.Publisher, sub message.Subscriber) func(t *testing.T) {
	topic := "hung-nack-test"
	return func(t *testing.T) {
		t.Parallel()

		original := message.NewMessage("hungMessage", []byte("payload"))
		err := pub.Publish(
			topic,
			original,
			message.NewMessage("tailMessage", []byte("payload2")),
		)
		if err != nil {
			t.Fatal(err)
		}

		// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
		msgs, err := sub.Subscribe(context.TODO(), topic)
		if err != nil {
			t.Fatal(err)
		}

		select {
		case msg := <-msgs:
			if msg == nil {
				t.Fatal("message is nil")
			}
			if !bytes.Equal(msg.Payload, original.Payload) {
				t.Errorf("expected payload %q, got %q", original.Payload, msg.Payload)
			}
			// <-time.After(time.Second)
			msg.Nack()
		case <-time.After(time.Second):
			t.Fatal("first message was not received before nack")
		}

		select {
		case msg := <-msgs:
			if msg == nil {
				t.Fatal("message is nil")
			}
			if msg.UUID != original.UUID {
				t.Errorf("expected payload %q, got %q", original.UUID, msg.UUID)
			}
			// <-time.After(time.Second)
			msg.Ack()
		case <-time.After(time.Second):
			t.Fatal("second message was not received after nack")
		}

		select {
		case tail := <-msgs:
			tail.Ack()
			if tail.UUID != "tailMessage" {
				t.Errorf("expected payload %q, got %q", "tailMessage", tail.UUID)
			}
		case <-time.After(time.Second):
			t.Fatal("tail message was not never received")
		}
	}
}
