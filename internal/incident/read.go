package incident

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// FoldAll builds the incident read model from the whole journal without
// touching watermarks — the read-only fold behind `tq incidents`. Unlike
// NewPolicy it reacts to nothing and persists nothing, so CLI reads never
// move a consumer cursor.
func FoldAll(ctx context.Context, store queue.Store) (*State, error) {
	const page = 500

	state := NewState()

	for {
		facts, err := store.Facts(ctx, state.LastSeq(), page)
		if err != nil {
			return nil, fmt.Errorf("incident: fold: %w", err)
		}

		for _, f := range facts {
			state.Apply(f)
		}

		if len(facts) < page {
			return state, nil
		}
	}
}
