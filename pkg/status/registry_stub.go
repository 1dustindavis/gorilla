// Without a darwin specific build, go tools will try to include Windows libraries and fail

//go:build !windows
// +build !windows

package status

func getUninstallKeys() (map[string]RegistryApplication, error) {
	return nil, nil
}
