package wmsqlitezombiezen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type subscription struct {
	Connection   *sqlite.Conn
	pollTicker   *time.Ticker
	lockTicker   *time.Ticker
	lockDuration time.Duration
	nackChannel  func() <-chan time.Time

	stmtLockConsumerGroup   *sqlite.Stmt
	stmtExtendLock          *sqlite.Stmt
	stmtNextMessageBatch    *sqlite.Stmt
	stmtAcknowledgeMessages *sqlite.Stmt

	lockedOffset    int64
	lastAckedOffset int64
	destination     chan *message.Message
	bufferPool      *sync.Pool
	logger          watermill.LoggerAdapter
}

type rawMessage struct {
	Offset   int64
	UUID     string
	Payload  []byte
	Metadata message.Metadata
}

// NextBatch fetches the next batch of messages from the database.
// Returns [ErrConsumerGroupIsLocked] if row lock could not be acquired.
func (s *subscription) NextBatch() (batch []rawMessage, err error) {
	// TODO: or ExclusiveTransaction?
	closeTransaction, err := sqlitex.ImmediateTransaction(s.Connection)
	if err != nil {
		return nil, err
	}
	// closeTransaction := sqlitex.Transaction(s.Connection)
	defer func() {
		if len(batch) == 0 && err == nil {
			// cancel writing the lock to the database
			// when no messages were fetched
			// to avoid a database write on every poll interval
			closeTransaction(&errEmptyBatch)
			return
		}
		closeTransaction(&err)
	}()

	if err = s.stmtLockConsumerGroup.Reset(); err != nil {
		return nil, err
	}
	ok, err := s.stmtLockConsumerGroup.Step()
	if err != nil {
		return nil, fmt.Errorf("unable to read offset_acked value: %w", err)
	}
	if !ok {
		return nil, ErrConsumerGroupIsLocked
	}
	s.lockedOffset = s.stmtLockConsumerGroup.ColumnInt64(0)
	s.lastAckedOffset = s.lockedOffset
	ok, err = s.stmtLockConsumerGroup.Step()
	if err != nil {
		return nil, fmt.Errorf("unable to finish reading offset_acked value: %w", err)
	}
	if ok {
		return nil, ErrMoreRowStepsThanExpected
	}

	if err = s.stmtNextMessageBatch.Reset(); err != nil {
		return nil, err
	}
	s.stmtNextMessageBatch.BindInt64(1, s.lockedOffset)
	b := s.bufferPool.Get().(*bytes.Buffer)
	defer s.bufferPool.Put(b)
	for {
		ok, err = s.stmtNextMessageBatch.Step()
		if err != nil {
			return nil, fmt.Errorf("unable to read message row: %w", err)
		}
		if !ok {
			break
		}
		next := rawMessage{
			Offset: s.stmtNextMessageBatch.ColumnInt64(0),
			UUID:   s.stmtNextMessageBatch.ColumnText(1),
		}
		b.Reset() // might be full from pool; note that pool may leak message metadata
		if _, err = io.Copy(b, s.stmtNextMessageBatch.ColumnReader(2)); err != nil {
			return nil, fmt.Errorf("unable to read message payload: %w", err)
		}
		if b.Len() == 0 {
			// Clarification: https://github.com/ThreeDotsLabs/watermill/issues/565#issuecomment-2885938295
			next.Payload = []byte{}
		} else {
			next.Payload = slices.Clone(b.Bytes())
			b.Reset()
		}
		if _, err = io.Copy(b, s.stmtNextMessageBatch.ColumnReader(3)); err != nil {
			return nil, fmt.Errorf("unable to read message metadata: %w", err)
		}

		if err = json.Unmarshal(b.Bytes(), &next.Metadata); err != nil {
			return nil, fmt.Errorf("unable to parse message metadata JSON: %w", err)
		}
		batch = append(batch, next)
	}

	return batch, nil
}

func (s *subscription) ExtendLock() (err error) {
	if err = s.stmtExtendLock.Reset(); err != nil {
		return err
	}
	s.stmtExtendLock.BindInt64(1, s.lastAckedOffset)
	s.stmtExtendLock.BindInt64(2, s.lockedOffset)

	ok, err := s.stmtExtendLock.Step()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("lock extension did not return any rows")
	}
	ok, err = s.stmtExtendLock.Step()
	if err != nil {
		return err
	}
	if ok {
		return ErrMoreRowStepsThanExpected
	}
	s.lockTicker.Reset(s.lockDuration)
	s.lockedOffset = s.lastAckedOffset
	return nil
}

func (s *subscription) ReleaseLock() (err error) {
	if err = s.stmtAcknowledgeMessages.Reset(); err != nil {
		return err
	}
	s.stmtAcknowledgeMessages.BindInt64(1, s.lastAckedOffset)
	s.stmtAcknowledgeMessages.BindInt64(2, s.lockedOffset)

	ok, err := s.stmtAcknowledgeMessages.Step()
	if err != nil {
		return err
	}
	if ok {
		return ErrMoreRowStepsThanExpected
	}
	return nil
}

func (s *subscription) Send(parent context.Context, next rawMessage) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	s.lockTicker.Reset(s.lockDuration)
	for {
		msg := message.NewMessage(next.UUID, next.Payload)
		msg.Metadata = next.Metadata
		msg.SetContext(ctx) // required for passing official PubSub test tests.TestMessageCtx

		select { // wait for message emission
		case <-ctx.Done():
			return nil
		case <-s.lockTicker.C:
			return s.ReleaseLock()
		case s.destination <- msg:
		}

	waitForMessageAcknowledgement:
		select {
		case <-ctx.Done():
			msg.Nack()
			return nil
		case <-s.lockTicker.C:
			if err := s.ExtendLock(); err != nil {
				return err
			}
			goto waitForMessageAcknowledgement
		case <-msg.Acked():
			s.lastAckedOffset = next.Offset
			return nil
		case <-s.nackChannel():
			s.logger.Debug("message took too long to be acknowledged", nil)
			msg.Nack()
			if err := s.ExtendLock(); err != nil {
				return err
			}
		case <-msg.Nacked():
		}
	}
}

func (s *subscription) Run(ctx context.Context) {
	var (
		batch []rawMessage
		err   error
	)

	for {
		select {
		case <-ctx.Done():
			if err = errors.Join(
				ctx.Err(),
				s.stmtLockConsumerGroup.Finalize(),
				s.stmtExtendLock.Finalize(),
				s.stmtNextMessageBatch.Finalize(),
				s.stmtAcknowledgeMessages.Finalize(),
				s.Connection.Close(),
			); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("subscription ended with error", err, nil)
			}
			return
		case <-s.pollTicker.C:
		}

		batch, err = s.NextBatch()
		if err != nil {
			if !errors.Is(err, ErrConsumerGroupIsLocked) && !isInterrupt(err) {
				s.logger.Error("next message batch query failed", err, nil)
			}
			continue
		}
		if len(batch) == 0 {
			continue // the lock is never set on empty batches
		}

		for _, next := range batch {
			if err = s.Send(ctx, next); err != nil {
				if !isInterrupt(err) {
					s.logger.Error("failed to process queued message", err, nil)
				}
				continue
			}
		}

		if err = s.ReleaseLock(); err != nil {
			if !isInterrupt(err) {
				s.logger.Error("failed to acknowledge processed messages", err, nil)
			}
		}
	}
}

func isInterrupt(err error) bool {
	if sqlite.ErrCode(err) == sqlite.ResultInterrupt {
		return true
	}
	return errors.Is(err, ErrConsumerGroupIsLocked)
}
