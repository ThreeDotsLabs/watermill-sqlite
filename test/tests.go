package tests

import (
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
)

type PubSubFixture func(t *testing.T, consumerGroup string) (message.Publisher, message.Subscriber)

func (f PubSubFixture) WithConsumerGroup(consumerGroup string) func(t *testing.T) (message.Publisher, message.Subscriber) {
	return func(t *testing.T) (message.Publisher, message.Subscriber) {
		pub, sub := f(t, consumerGroup)
		return pub, sub
	}
}
