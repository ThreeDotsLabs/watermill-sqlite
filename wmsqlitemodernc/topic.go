package wmsqlitemodernc

import (
	"context"
	"fmt"
	"regexp"
)

var disallowedTopicCharacters = regexp.MustCompile(`[^A-Za-z0-9\-\$\:\.\_]`)

// validateTopicName checks if the topic name contains any characters which could be unsuitable for the SQL Pub/Sub.
// Topics are translated into SQL tables and patched into some queries, so this is done to prevent injection as well.
func validateTopicName(topic string) error {
	if disallowedTopicCharacters.MatchString(topic) {
		return fmt.Errorf("invalid topic name %q: %w", topic, ErrInvalidTopicName)
	}
	if topic == "" {
		return fmt.Errorf("empty topic name %q: %w", topic, ErrInvalidTopicName)
	}
	return nil
}

func CreateTopicAndOffsetsTablesIfAbsent(ctx context.Context, db SQLiteConnection, messagesTableName, offsetsTableName string) (err error) {
	if err = validateTopicName(messagesTableName); err != nil {
		return err
	}
	// adding UNIQUE(uuid) constraint slows the driver down without benefit
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS '`+messagesTableName+`' (
		'offset' INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		uuid TEXT NOT NULL,
		created_at TEXT NOT NULL,
		payload BLOB NOT NULL,
		metadata JSON NOT NULL
	);`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS '`+offsetsTableName+`' (
		consumer_group TEXT NOT NULL,
		offset_acked INTEGER NOT NULL,
		locked_until INTEGER NOT NULL,
		PRIMARY KEY(consumer_group)
	);`)
	if err != nil {
		return err
	}
	return err
}
