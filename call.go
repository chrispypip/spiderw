package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/logging"
)

// delegate runs a public-API operation against a lazily-resolved backend.
//
// It factors out the boilerplate shared by every public accessor:
//
//   - resolve obtains the backend facade (the coreAdapter/coreDaemon nil-check),
//     returning an already-wrapped public error on failure;
//   - fn performs the actual backend call and any result mapping;
//   - failures are logged once and wrapped via wrapPublicError;
//   - entry/exit are traced at debug level with a uniform "op" field.
//
// B is the backend interface type (core.AdapterIface, core.DaemonIface); T is
// the public return type.
func delegate[B any, T any](
	ctx context.Context,
	op string,
	resolve func(ctx context.Context, op string) (B, error),
	fn func(ctx context.Context, backend B) (T, error),
) (T, error) {
	var zero T
	log := logging.FromContext(ctx)

	backend, err := resolve(ctx, op)
	if err != nil {
		return zero, err
	}

	log.Debug(ctx, "op start", "op", op)

	v, err := fn(ctx, backend)
	if err != nil {
		log.Error(ctx, "op failed", "op", op, "err", err)
		return zero, wrapPublicError(op, err)
	}

	log.Debug(ctx, "op ok", "op", op)
	return v, nil
}

// do is the value-less variant of delegate for operations that return only an
// error (e.g. setters).
func do[B any](
	ctx context.Context,
	op string,
	resolve func(ctx context.Context, op string) (B, error),
	fn func(ctx context.Context, backend B) error,
) error {
	_, err := delegate(ctx, op, resolve, func(ctx context.Context, b B) (struct{}, error) {
		return struct{}{}, fn(ctx, b)
	})
	return err
}
