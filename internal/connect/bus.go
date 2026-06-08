package connect

import "context"

// Session creates a Wiring instance using the session D-Bus bus.
func Session(ctx context.Context) (*Wiring, error) {
	return newConn(ctx, false)
}

// System creates a Wiring instance using the system D-Bus bus.
func System(ctx context.Context) (*Wiring, error) {
	return newConn(ctx, true)
}
