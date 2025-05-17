package wmsqlitemodernc

import (
	"context"
	"database/sql"
	"testing"
)

func TestTopicTableCreation(t *testing.T) {
	// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	db, err := sql.Open("sqlite", ":memory:?journal_mode=WAL&busy_timeout=1000&cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	})

	err = CreateTopicAndOffsetsTablesIfAbsent(
		ctx,
		db,
		"messagesTableName",
		"offsetsTableName",
	)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryContext(ctx, `
	SELECT
	    name
	FROM
	    sqlite_schema
	WHERE
	    type ='table' AND
	    name NOT LIKE 'sqlite_%';`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
		t.Log("Found table:", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	// t.Fatal(tables)
	if len(tables) != 2 {
		t.Fatal("Expected 2 tables, got", len(tables))
	}
}
