package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// WSC (SimpleConfiguration) mock behavior. Enrollment succeeds immediately (the
// mock station is already connected); a sentinel PIN lets integration tests
// exercise iwd's WSC error taxonomy deterministically.
const (
	// wscGeneratedPin is the PIN GeneratePin returns. Its exact digits are what
	// integration tests assert flows back to the caller.
	wscGeneratedPin = "12345670"

	// wscNoCredentialsPin, passed to StartPin, makes the mock report the WSC
	// NoCredentials error so tests can assert the matchable sentinel end to end.
	// It is a valid 8-digit length so it passes the client-side format check and
	// reaches the mock (where iwd would otherwise reject it after the exchange).
	wscNoCredentialsPin = "00000000"
)

// PushButton implements net.connman.iwd.SimpleConfiguration.PushButton. The mock
// treats PushButton (PBC) enrollment as succeeding immediately.
func (d *Device) PushButton() *dbus.Error {
	if !d.HasStation {
		return dbus.MakeFailedError(fmt.Errorf("device has no simple configuration interface"))
	}
	return nil
}

// GeneratePin implements net.connman.iwd.SimpleConfiguration.GeneratePin,
// returning a fixed 8-digit PIN.
func (d *Device) GeneratePin() (string, *dbus.Error) {
	if !d.HasStation {
		return "", dbus.MakeFailedError(fmt.Errorf("device has no simple configuration interface"))
	}
	return wscGeneratedPin, nil
}

// StartPin implements net.connman.iwd.SimpleConfiguration.StartPin. It succeeds
// for a normal PIN; the sentinel wscNoCredentialsPin reports the WSC
// NoCredentials error.
func (d *Device) StartPin(pin string) *dbus.Error {
	if !d.HasStation {
		return dbus.MakeFailedError(fmt.Errorf("device has no simple configuration interface"))
	}
	if pin == wscNoCredentialsPin {
		return dbus.NewError(iwdbus.IwdErrorWSCNoCredentials, []interface{}{"no usable credentials obtained"})
	}
	return nil
}

// Cancel implements net.connman.iwd.SimpleConfiguration.Cancel.
func (d *Device) Cancel() *dbus.Error {
	if !d.HasStation {
		return dbus.MakeFailedError(fmt.Errorf("device has no simple configuration interface"))
	}
	return nil
}
