package postgres_test

import (
	"context"
	"testing"

	tq "github.com/larsartmann/go-taskqueue/queue/postgres"
)

func TestOpenWithPoolNilPool(t *testing.T) {
	if _, err := tq.OpenWithPool(context.Background(), nil); err == nil {
		t.Fatal("nil pool must be refused")
	}
}
