//go:build integration

package integration_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	signalTimeout = 1 * time.Second
	pollInterval  = 10 * time.Millisecond
)

func runSpider(t *testing.T, args ...string) (string, error) {
	t.Helper()

	// The CLI defaults to the system bus; --session points it at the session
	// bus, where the iwd mock under test is registered.
	cliArgs := append([]string{"run", "github.com/chrispypip/spiderw/cmd/spiderw", "--session"}, args...)
	out, err := exec.Command("go", cliArgs...).CombinedOutput()
	outString := string(out)
	return outString, err
}

// runSpiderJSON runs the spiderw CLI with --json enabled and returns the parsed
// JSON object.
//
// This helper intentionally does not change CLI behavior; it only opts into the
// existing JSON output mode. Errors are still returned as the underlying
// exec.Command error, and the returned raw output includes stderr/stdout as
// produced by CombinedOutput().
func runSpiderJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()

	out, err := runSpider(t, append([]string{"--json"}, args...)...)
	if err != nil {
		return nil, out, err
	}

	obj := extractJSONObject(t, out)
	m := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(obj), &m), "failed to parse JSON output: %s", out)
	return m, out, nil
}

func extractJSONObject(t *testing.T, out string) string {
	t.Helper()

	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	require.GreaterOrEqual(t, start, 0, "missing '{' in output:\n%s", out)
	require.GreaterOrEqual(t, end, 0, "missing '}' in output:\n%s", out)
	require.Greater(t, end, start, "invalid JSON braces in output:\n%s", out)
	return out[start : end+1]
}

func jsonGetString(t *testing.T, m map[string]any, key string) string {
	t.Helper()

	v, ok := m[key]
	require.True(t, ok, "missing key %q in JSON: %#v", key, m)
	s, ok := v.(string)
	require.True(t, ok, "key %q expected string, got %T (%v)", key, v, v)
	return s
}

func jsonGetBool(t *testing.T, m map[string]any, key string) bool {
	t.Helper()

	v, ok := m[key]
	require.True(t, ok, "missing key %q in JSON: %#v", key, m)
	b, ok := v.(bool)
	require.True(t, ok, "key %q expected bool, got %T (%v)", key, v, v)
	return b
}

func jsonGetArray(t *testing.T, m map[string]any, key string) []any {
	t.Helper()

	v, ok := m[key]
	require.True(t, ok, "missing key %q in JSON: %#v", key, m)
	a, ok := v.([]any)
	require.True(t, ok, "key %q expected list, got %T (%v)", key, v, v)
	return a
}

func mustContain(t *testing.T, out, substr string) {
	t.Helper()

	require.Contains(t, out, substr, "output:\n%s", out)
}

func mustContainAll(t *testing.T, out string, substrs []string) {
	t.Helper()

	for _, s := range substrs {
		mustContain(t, out, s)
	}
}

func requireFired(t *testing.T, ch <-chan struct{}, msg ...string) {
	t.Helper()

	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, signalTimeout, pollInterval, msg)
}

func requireNotFired(t *testing.T, ch <-chan struct{}, msg ...string) {
	t.Helper()

	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return false
		default:
			return true
		}
	}, signalTimeout, pollInterval, msg)
}
