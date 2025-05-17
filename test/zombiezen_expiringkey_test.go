package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen"
)

func TestExpiringKeyRepository_zombiezen(t *testing.T) {
	// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	conn := newTestConnectionZombiezen(t, ":memory:")
	r, finalizer, err := wmsqlitezombiezen.NewExpiringKeyRepository(wmsqlitezombiezen.ExpiringKeyRepositoryConfiguration{
		Connection: conn,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := finalizer(); err != nil {
			t.Fatal(err)
		}
	})

	isDuplicate, err := r.IsDuplicate(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if isDuplicate {
		t.Fatal("key should not be duplicate")
	}
	isDuplicate, err = r.IsDuplicate(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if !isDuplicate {
		t.Fatal("key should be duplicate")
	}

	if err = r.(interface {
		CleanUp(time.Time) error
	}).CleanUp(time.Time{}); err != nil {
		t.Fatal(err)
	}
}
