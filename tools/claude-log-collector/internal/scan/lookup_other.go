//go:build !windows

package scan

func lookupUserMachineEnv(string) string { return "" }
