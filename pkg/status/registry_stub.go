// Without a darwin specific build, go tools will try to include Windows libraries and fail

// +build !windows

package status

func getUninstallKeys() (map[string]RegistryApplication, error) {
	return nil, nil
}
