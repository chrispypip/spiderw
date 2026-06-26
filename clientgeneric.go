package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/logging"
)

// clientObject is the shared body of the single-object Client.<Object>
// constructors (Adapter, Device, BasicServiceSet, Network, KnownNetwork). Each
// public method is a one-liner that supplies its op name, the *connect.Wiring
// constructor (a method expression such as (*connect.Wiring).NewNetwork), and the
// public wrapper (such as newNetwork).
func clientObject[CoreI any, Pub any](
	c *Client,
	ctx context.Context,
	op, path string,
	build func(*connect.Wiring, context.Context, string) (CoreI, error),
	wrap func(CoreI, string) *Pub,
) (*Pub, error) {
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op, "path", path)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreObj, err := build(c.wire, ctx, path)
	if err != nil {
		log.Error(ctx, "object wiring failed", "op", op, "path", path, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := wrap(coreObj, path)
	if pub == nil {
		log.Error(ctx, "object wrapper unexpectedly nil", "op", op, "path", path)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return pub, nil
}
