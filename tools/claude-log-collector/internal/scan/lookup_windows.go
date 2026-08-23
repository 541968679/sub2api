//go:build windows

package scan

import "golang.org/x/sys/windows/registry"

func lookupUserMachineEnv(name string) string {
	if v := readRegistryEnv(registry.CURRENT_USER, `Environment`, name); v != "" {
		return v
	}
	if v := readRegistryEnv(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, name); v != "" {
		return v
	}
	return ""
}

func readRegistryEnv(root registry.Key, path, name string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	val, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return val
}
