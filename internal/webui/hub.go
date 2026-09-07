package webui

import (
	"context"

	"github.com/larsartmann/go-sse"
)

// subscriberBufferSize is the per-client event buffer. Overflow events are
// dropped for that client; the next change notification triggers a full
// server-rendered snapshot, so the page self-heals.
const subscriberBufferSize = 128

// Hub fans out change notifications to all connected SSE clients.
// Notifications carry no payload — every one triggers a server-side
// re-render and a fresh full snapshot to each client (projection, not
// client state). The event ID is the journal watermark so browsers resume
// with a meaningful Last-Event-ID.
type Hub struct {
	bc *sse.Broadcaster[sse.Event]
}

// NewHub creates a ready-to-use Hub.
func NewHub() *Hub {
	return &Hub{
		bc: sse.NewBroadcaster[sse.Event](sse.WithBufferSize[sse.Event](subscriberBufferSize)),
	}
}

// Notify broadcasts a change notification to every subscriber. seq is the
// journal watermark the notification represents.
func (h *Hub) Notify(seq int64) {
	h.bc.Broadcast(sse.Event{
		Event: "tick",
		ID:    sse.NewEventID(formatSeq(seq)),
	})
}

// Subscribe returns a channel receiving notifications.
func (h *Hub) Subscribe() <-chan sse.Event {
	return h.bc.Subscribe()
}

// Unsubscribe removes a subscriber channel.
func (h *Hub) Unsubscribe(ch <-chan sse.Event) {
	h.bc.Unsubscribe(ch)
}

// ClientCount returns the number of connected subscribers.
func (h *Hub) ClientCount() int {
	return h.bc.SubscriberCount()
}

// Shutdown drains the broadcaster.
func (h *Hub) Shutdown(ctx context.Context) error {
	return h.bc.Shutdown(ctx)
}
