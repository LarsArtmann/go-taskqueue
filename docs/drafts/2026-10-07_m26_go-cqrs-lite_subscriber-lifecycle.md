# M26 draft — go-cqrs-lite issue filing (solicited, f-list)

Title: `event.Subscriber has no shutdown hook — "smallest interface" consumers can't stop their goroutines`

Body:

TL;DR: `event.Subscriber` (master `event/bus.go:28-31`) only carries `Subscribe`/`SubscribeAll`, so a consumer holding the smallest interface has no way to stop the goroutine its handler runs in. The ask: a documented lifecycle convention for subscribers — e.g. the same optional-`io.Closer` note `Bus` already carries.

## Problem

`Bus` documents cleanup by type-assertion (`if c, ok := bus.(io.Closer); ok { c.Close() }`), and the doc comment steers consumers toward accepting only `event.Subscriber` ("only subscribes — never publishes"). But `Subscriber` itself has no cleanup contract:

```go
// event/bus.go, master as of 2026-10-07
type Subscriber interface {
	Subscribe(eventType Type, handler Handler) error
	SubscribeAll(handler Handler) error
}
```

A consumer who follows that advice holds no handle to stop the dispatch loop `SubscribeAll` starts.

We hit this directly in go-taskqueue: we tail the journal through a subscriber, and `projectionhost`'s own guidance for live tailing is "poll periodically by calling Start again" (`projectionhost/v4/host.go`, v4.5.3) — each start spawns fresh workers, so every worker generation that re-subscribed spawned another goroutine. The only stop point was `ProjectionHost.Close`; the subscriber had no shutdown signal of its own.

## What we do today

`internal/readmodel/host.go:344` (go-taskqueue, v0.3.3): our `tailSubscriber` implements `event.Subscriber` with its own `stop` channel + `sync.WaitGroup`, and the serve wiring reaches around the interface to the concrete type for shutdown. That works only because we own the concrete type. The leak history before that fix (tickers on `context.Background()`, one per worker generation per restart) is exactly the failure mode the interface permits today.

## Goal

As a consumer accepting `event.Subscriber`, I want a documented way to stop the subscription on shutdown — an optional interface convention (`if s, ok := sub.(interface{ Close() error }); ok { s.Close() }`) documented on `Subscriber`, mirroring the note `Bus` carries. Either that or any shape you prefer; the contract just shouldn't stay implicit, so every library doesn't invent its own stop channel.

No behavior change requested for existing implementations — a documented convention would already fix the consumer side.

💘 Generated with Crush
