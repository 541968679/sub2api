package scan

import (
	"os"
	"strings"
)

// Lookup resolves an environment variable name.
type Lookup func(name string) string

// LookupProcessUserMachine checks the current process environment first,
// then the Windows user (HKCU) and machine (HKLM) Environment stores.
func LookupProcessUserMachine(name string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return strings.TrimSpace(lookupUserMachineEnv(name))
}

// LookupFromMap returns a lookup that only reads the given map (for tests).
func LookupFromMap(m map[string]string) Lookup {
	return func(name string) string {
		if m == nil {
			return ""
		}
		return strings.TrimSpace(m[name])
	}
}
