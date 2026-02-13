package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"go.yaml.in/yaml/v4"
)

var (
	manifestGet = manifest.Get
	mkdirAll    = os.MkdirAll
)

type Command struct {
	Action string   `json:"action"`
	Items  []string `json:"items,omitempty"`
}

type CommandResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Items   []string `json:"items,omitempty"`
}

func parseCommandSpec(spec string) (Command, error) {
	var cmd Command
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return cmd, errors.New("service command cannot be empty")
	}

	parts := strings.SplitN(spec, ":", 2)
	cmd.Action = strings.ToLower(strings.TrimSpace(parts[0]))
	if len(parts) == 2 {
		items := strings.Split(parts[1], ",")
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				cmd.Items = append(cmd.Items, item)
			}
		}
	}
	return cmd, validateCommand(cmd)
}

func validateCommand(cmd Command) error {
	switch cmd.Action {
	case "run":
		if len(cmd.Items) != 0 {
			return errors.New("run action does not support items")
		}
	case "get-service-manifest", "get-optional-items":
		if len(cmd.Items) != 0 {
			return fmt.Errorf("%s action does not support items", cmd.Action)
		}
	case "install", "remove":
		if len(cmd.Items) == 0 {
			return fmt.Errorf("%s action requires at least one item", cmd.Action)
		}
	default:
		return fmt.Errorf("unsupported service action %q", cmd.Action)
	}

	return nil
}

func SendCommand(cfg config.Configuration, spec string) (CommandResponse, error) {
	cmd, err := parseCommandSpec(spec)
	if err != nil {
		return CommandResponse{}, err
	}
	return sendCommand(cfg, cmd)
}

func serviceInstallArgs(configPath string) []string {
	return []string{"-c", configPath, "-service"}
}

func executeCommand(cfg config.Configuration, cmd Command, managedRun func(config.Configuration) error) (CommandResponse, error) {
	switch cmd.Action {
	case "run":
		return CommandResponse{Status: "ok"}, managedRun(cfg)
	case "install":
		if err := addServiceManagedInstalls(cfg, cmd.Items); err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Status: "ok"}, managedRun(cfg)
	case "remove":
		if err := removeServiceManagedInstalls(cfg, cmd.Items); err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Status: "ok"}, nil
	case "get-service-manifest":
		items, err := listServiceManagedInstalls(cfg)
		if err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Status: "ok", Items: items}, nil
	case "get-optional-items":
		items, err := getOptionalItems(cfg)
		if err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Status: "ok", Items: items}, nil
	default:
		return CommandResponse{}, fmt.Errorf("unsupported service action %q", cmd.Action)
	}
}

func serviceLocalManifestPath(cfg config.Configuration) string {
	return filepath.Join(cfg.AppDataPath, "service-manifest.yaml")
}

func listServiceManagedInstalls(cfg config.Configuration) ([]string, error) {
	item, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return nil, err
	}
	return item.Installs, nil
}

func addServiceManagedInstalls(cfg config.Configuration, items []string) error {
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return err
	}

	for _, item := range items {
		if !slices.Contains(entry.Installs, item) {
			entry.Installs = append(entry.Installs, item)
		}
	}
	slices.Sort(entry.Installs)

	return saveServiceLocalManifest(cfg, entry)
}

func removeServiceManagedInstalls(cfg config.Configuration, items []string) error {
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(entry.Installs))
	for _, existing := range entry.Installs {
		if !slices.Contains(items, existing) {
			filtered = append(filtered, existing)
		}
	}
	entry.Installs = filtered
	return saveServiceLocalManifest(cfg, entry)
}

func loadServiceLocalManifest(cfg config.Configuration) (manifest.Item, error) {
	path := serviceLocalManifestPath(cfg)
	defaultManifest := manifest.Item{
		Name:     "service-manifest",
		Installs: []string{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultManifest, nil
		}
		return manifest.Item{}, fmt.Errorf("unable to read service local manifest %s: %w", path, err)
	}

	entry := defaultManifest
	if err := yaml.Unmarshal(data, &entry); err != nil {
		return manifest.Item{}, fmt.Errorf("unable to parse service local manifest %s: %w", path, err)
	}
	if entry.Name == "" {
		entry.Name = defaultManifest.Name
	}
	return entry, nil
}

func saveServiceLocalManifest(cfg config.Configuration, entry manifest.Item) error {
	path := serviceLocalManifestPath(cfg)
	if err := mkdirAll(filepath.Clean(filepath.Dir(path)), 0755); err != nil {
		return fmt.Errorf("unable to create local manifest directory: %w", err)
	}

	entry.Includes = nil
	entry.Uninstalls = nil
	entry.Updates = nil
	entry.Catalogs = nil

	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("unable to encode service local manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("unable to write service local manifest %s: %w", path, err)
	}
	return nil
}

func getOptionalItems(cfg config.Configuration) ([]string, error) {
	manifests, _ := manifestGet(cfg)
	options := make([]string, 0)
	seen := make(map[string]bool)
	for _, m := range manifests {
		for _, item := range m.OptionalInstalls {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			options = append(options, item)
		}
	}
	slices.Sort(options)
	return options, nil
}
