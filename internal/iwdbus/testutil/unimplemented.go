// Package testutil provides iwdbus test fakes and embeddable stubs.
package testutil

// UnimplementedIwdbusDaemon is a test-only embeddable stub for fakes that
// implement iwdbus daemon interfaces.
//
// Embed this type in a fake when the fake should only implement the subset of
// daemon behavior used by a specific test. Add panicking method stubs here only
// when a method is intentionally optional for most fakes but still needed to
// satisfy an interface in some tests. A panic from one of those stubs means the
// test exercised behavior that its fake did not explicitly implement.
type UnimplementedIwdbusDaemon struct{}
