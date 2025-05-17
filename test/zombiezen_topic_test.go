package tests

import (
	"testing"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func TestTopicTableCreation(t *testing.T) {
	conn := newTestConnectionZombiezen(t, ":memory:")

	err := wmsqlitezombiezen.CreateTopicAndOffsetsTablesIfAbsent(
		conn,
		"messagesTableName",
		"offsetsTableName",
	)
	if err != nil {
		t.Fatal("unable to create topic and offsets tables", err)
	}

	tables := make([]string, 0)
	err = sqlitex.ExecuteTransient(
		conn,
		`SELECT name FROM sqlite_schema
		WHERE type ='table' AND name NOT LIKE 'sqlite_%';`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				t.Log("Found table:", stmt.ColumnText(0))
				tables = append(tables, stmt.ColumnText(0))
				return nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	if len(tables) != 2 {
		t.Fatal("Expected 2 tables, got", len(tables))
	}
}
