package wmsqlitemodernc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

type subscription struct {
	DB           SQLiteDatabase
	pollTicker   *time.Ticker
	lockTicker   *time.Ticker
	lockDuration time.Duration
	nackChannel  func() <-chan time.Time

	sqlLockConsumerGroup   string
	sqlExtendLock          string
	sqlNextMessageBatch    string
	sqlAcknowledgeMessages string

	lockedOffset    int64
	lastAckedOffset int64
	destination     chan *message.Message
	logger          watermill.LoggerAdapter
}

type rawMessage struct {
	Offset   int64
	UUID     string
	Payload  []byte
	Metadata message.Metadata
}

func (s *subscription) NextBatch(ctx context.Context) (batch []rawMessage, err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			if len(batch) == 0 {
				// cancel writing the lock to the database
				// when no messages were fetched
				// to avoid a database write on every poll interval
				err = tx.Rollback()
				return
			}
			err = tx.Commit()
		}
		err = errors.Join(err, tx.Rollback())
		if errors.Is(err, sql.ErrTxDone) {
			// never return [sql.ErrTxDone] because it will break the official acceptance tests
			// when they are run with -count=5 or more
			//
			// since SQLite is basically a file handle, the transaction gets stuck if it does not complete
			// another subscriber inserts itself into the transaction because it is recorded into the
			// write-ahead-journal. Just a guess. Concealing the error removed dead locks.
			err = nil
		}
	}()

	// Transaction execution and query operations must be context-less. Otherwise, a message occasionally will get lost, because the transaction will not be committed because one of the operations will not run with a cancelled context. Strange behavior, but it is proven by TestContinueAfterSubscribeClose with run with -count=5 or more.
	lock := tx.QueryRow(s.sqlLockConsumerGroup)
	if err = lock.Err(); err != nil {
		return nil, fmt.Errorf("unable to acquire row lock: %w", err)
	}
	if err = lock.Scan(&s.lockedOffset); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, tx.Commit() // unable  to acquire row lock
		}
		return nil, fmt.Errorf("unable to scan offset_acked value: %w", err)
	}
	s.lastAckedOffset = s.lockedOffset

	rows, err := tx.Query(s.sqlNextMessageBatch, s.lockedOffset) // contextless
	if err != nil {
		return nil, fmt.Errorf("unable to query next message batch: %w", err)
	}
	return buildBatch(rows)
}

func buildBatch(rows *sql.Rows) (batch []rawMessage, err error) {
	defer func() {
		err = errors.Join(rows.Close())
	}()
	rawMetadata := []byte{} // TODO: use buffer pool
	for rows.Next() {
		next := rawMessage{}
		if err = rows.Scan(&next.Offset, &next.UUID, &next.Payload, &rawMetadata); err != nil {
			return nil, err
		}
		if next.Payload == nil {
			// Clarification: https://github.com/ThreeDotsLabs/watermill/issues/565#issuecomment-2885938295
			next.Payload = []byte{}
		}
		if err = json.Unmarshal(rawMetadata, &next.Metadata); err != nil {
			return nil, fmt.Errorf("unable to parse metadata JSON: %w", err)
		}
		batch = append(batch, next)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return batch, nil
}

func (s *subscription) ExtendLock(ctx context.Context) error {
	row := s.DB.QueryRowContext(ctx, s.sqlExtendLock, s.lastAckedOffset, s.lockedOffset)
	if err := row.Err(); err != nil {
		return fmt.Errorf("unable to extend lock: %w", err)
	}
	s.lockTicker.Reset(s.lockDuration)
	s.lockedOffset = s.lastAckedOffset
	return nil
}

func (s *subscription) ReleaseLock(ctx context.Context) (err error) {
	_, err = s.DB.ExecContext(
		ctx,
		s.sqlAcknowledgeMessages,
		s.lastAckedOffset,
		s.lockedOffset,
	)
	return err
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
			return s.ReleaseLock(ctx)
		case s.destination <- msg:
		}

	waitForMessageAcknowledgement:
		select {
		case <-ctx.Done():
			msg.Nack()
			return nil
		case <-s.lockTicker.C:
			if err := s.ExtendLock(ctx); err != nil {
				return err
			}
			goto waitForMessageAcknowledgement
		case <-msg.Acked():
			s.lastAckedOffset = next.Offset
			return nil
		case <-s.nackChannel():
			s.logger.Debug("message took too long to be acknowledged", nil)
			msg.Nack()
			if err := s.ExtendLock(ctx); err != nil {
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
			return
		case <-s.pollTicker.C:
		}

		batch, err = s.NextBatch(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				s.logger.Error("next message batch query failed", err, nil)
			}
			continue
		}
		if len(batch) == 0 {
			continue // the lock is never set on empty batches
		}

		for _, next := range batch {
			if err = s.Send(ctx, next); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.logger.Error("failed to process queued message", err, nil)
				}
				continue
			}
		}

		if err = s.ReleaseLock(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				s.logger.Error("failed to acknowledge processed messages", err, nil)
			}
		}
	}
}
