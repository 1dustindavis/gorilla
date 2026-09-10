package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/1dustindavis/gorilla/pkg/appcatalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"go.yaml.in/yaml/v4"
)

var (
	manifestGet = manifest.Get
	mkdirAll    = os.MkdirAll
)

type Command struct {
	Action     string                `json:"action"`
	Items      []string              `json:"items,omitempty"`
	MutationID string                `json:"-"`
	RunConfig  *config.Configuration `json:"-"`
}

type CommandResponse struct {
	Status          string                `json:"status"`
	Message         string                `json:"message,omitempty"`
	Items           []string              `json:"items,omitempty"`
	OperationID     string                `json:"operationId,omitempty"`
	OptionalItems   []optionalItemDetails `json:"-"`
	RunConfig       *config.Configuration `json:"-"`
	CleanupPath     string                `json:"-"`
	ReusedOperation bool                  `json:"-"`
}

type actionDeniedError struct{ reason string }

func (e actionDeniedError) Error() string { return "action is not allowed: " + e.reason }

const (
	actionRun                   = "run"
	actionListOptionalInstalls  = "ListOptionalInstalls"
	actionInstallItem           = "InstallItem"
	actionRemoveItem            = "RemoveItem"
	actionStreamOperationStatus = "StreamOperationStatus"
)

func canonicalizeAction(action string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case strings.ToLower(actionRun):
		return actionRun, true
	case strings.ToLower(actionListOptionalInstalls):
		return actionListOptionalInstalls, true
	case strings.ToLower(actionInstallItem):
		return actionInstallItem, true
	case strings.ToLower(actionRemoveItem):
		return actionRemoveItem, true
	case strings.ToLower(actionStreamOperationStatus):
		return actionStreamOperationStatus, true
	default:
		return "", false
	}
}

func parseCommandSpec(spec string) (Command, error) {
	var cmd Command
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return cmd, errors.New("service command cannot be empty")
	}

	parts := strings.SplitN(spec, ":", 2)
	canonicalAction, ok := canonicalizeAction(parts[0])
	if !ok {
		return cmd, fmt.Errorf("unsupported service action %q", strings.TrimSpace(parts[0]))
	}
	cmd.Action = canonicalAction
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
	canonicalAction, ok := canonicalizeAction(cmd.Action)
	if !ok {
		return fmt.Errorf("unsupported service action %q", cmd.Action)
	}
	cmd.Action = canonicalAction

	switch cmd.Action {
	case actionRun:
		if len(cmd.Items) != 0 {
			return errors.New("run action does not support items")
		}
	case actionListOptionalInstalls:
		if len(cmd.Items) != 0 {
			return fmt.Errorf("%s action does not support items", cmd.Action)
		}
	case actionInstallItem, actionRemoveItem, actionStreamOperationStatus:
		if len(cmd.Items) != 1 {
			return fmt.Errorf("%s action requires exactly one argument", cmd.Action)
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

func serviceInstallArgs(configPath string, integrationTestServiceIdentity string) []string {
	args := []string{"-c", configPath, "-service"}
	if integrationTestServiceIdentity != "" {
		args = append(args, "-integration-test-service-identity", integrationTestServiceIdentity)
	}
	return args
}

func executeCommand(cfg config.Configuration, cmd Command, managedRun func(config.Configuration) error) (CommandResponse, error) {
	switch cmd.Action {
	case actionRun:
		if cmd.RunConfig != nil {
			cfg = *cmd.RunConfig
		}
		if err := reconcileServiceManagedInstalls(cfg); err != nil {
			return CommandResponse{}, fmt.Errorf("reconcile App Catalog install selections: %w", err)
		}
		return CommandResponse{Status: "ok"}, managedRun(cfg)
	case actionInstallItem:
		details, err := getOptionalItemDetails(cfg)
		if err != nil {
			return CommandResponse{}, err
		}
		item, ok := findOptionalItem(details, cmd.Items[0])
		if !ok {
			return CommandResponse{}, actionDeniedError{"not_optional"}
		}
		if !item.Contract.Actions.Install.Allowed {
			return CommandResponse{}, actionDeniedError{item.Contract.Actions.Install.Reason}
		}
		if err := addServiceManagedInstalls(cfg, cmd.Items); err != nil {
			return CommandResponse{}, err
		}
		operationID := strconv.FormatInt(time.Now().UnixNano(), 10)
		return CommandResponse{Status: "ok", OperationID: operationID}, nil
	case actionRemoveItem:
		details, err := getOptionalItemDetails(cfg)
		if err != nil {
			return CommandResponse{}, err
		}
		item, ok := findOptionalItem(details, cmd.Items[0])
		if !ok {
			return CommandResponse{}, actionDeniedError{"not_optional"}
		}
		if !item.Contract.Actions.Remove.Allowed {
			return CommandResponse{}, actionDeniedError{item.Contract.Actions.Remove.Reason}
		}
		operationID := strconv.FormatInt(time.Now().UnixNano(), 10)
		runCfg, cleanupPath, err := prepareOneTimeRemoval(cfg, cmd.Items[0], operationID, item.Contract.Observation.State != appcatalog.Absent)
		if err != nil {
			return CommandResponse{}, err
		}
		return CommandResponse{Status: "ok", OperationID: operationID, RunConfig: &runCfg, CleanupPath: cleanupPath}, nil
	case actionListOptionalInstalls:
		details, err := getOptionalItemDetails(cfg)
		if err != nil {
			return CommandResponse{}, err
		}
		items := make([]string, 0, len(details))
		for _, item := range details {
			items = append(items, item.Contract.ItemName)
		}
		return CommandResponse{Status: "ok", Items: items, OptionalItems: details}, nil
	case actionStreamOperationStatus:
		return CommandResponse{
			Status:  "ok",
			Message: "stream status is not yet implemented in the service",
		}, nil
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
	entry.Uninstalls = withoutItems(entry.Uninstalls, items)
	slices.Sort(entry.Installs)

	return saveServiceLocalManifest(cfg, entry)
}

func removeServiceManagedInstalls(cfg config.Configuration, items []string) error {
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return err
	}

	entry.Installs = withoutItems(entry.Installs, items)
	// User removal is a one-time request. Clear old service-generated uninstall
	// policy rather than persisting a desired-absent state.
	entry.Uninstalls = nil
	return saveServiceLocalManifest(cfg, entry)
}

func prepareOneTimeRemoval(cfg config.Configuration, item, operationID string, needsUninstall bool) (config.Configuration, string, error) {
	if !needsUninstall {
		return cfg, "", removeServiceManagedInstalls(cfg, []string{item})
	}
	dir := filepath.Join(cfg.AppDataPath, "operations")
	if err := mkdirAll(filepath.Clean(dir), 0755); err != nil {
		return cfg, "", err
	}
	path := filepath.Join(dir, operationID+"-remove.yaml")
	data, err := yaml.Marshal(manifest.Item{Name: "one-time-removal", Uninstalls: []string{item}})
	if err != nil {
		return cfg, "", err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return cfg, "", err
	}
	if err := removeServiceManagedInstalls(cfg, []string{item}); err != nil {
		_ = os.Remove(path)
		return cfg, "", err
	}
	runCfg := cfg
	runCfg.LocalManifests = append(append([]string(nil), cfg.LocalManifests...), path)
	return runCfg, path, nil
}

func clearLegacyServiceUninstalls(cfg config.Configuration) error {
	entry, err := loadServiceLocalManifest(cfg)
	if err != nil {
		return err
	}
	if len(entry.Uninstalls) == 0 {
		return nil
	}
	entry.Uninstalls = nil
	return saveServiceLocalManifest(cfg, entry)
}

func reconcileServiceManagedInstalls(cfg config.Configuration) error {
	selection, err := loadServiceLocalManifest(cfg)
	if err != nil || len(selection.Installs) == 0 {
		return err
	}
	manifests, _, err := manifestGet(cfg)
	if err != nil {
		return fmt.Errorf("retrieve manifests: %w", err)
	}
	_, requiredUninstalls := administratorRequirements(cfg, manifests, selection)
	_, err = reconcileServiceSelection(cfg, selection, requiredUninstalls)
	return err
}

func reconcileServiceSelection(cfg config.Configuration, selection manifest.Item, requiredUninstalls map[string]int) (manifest.Item, error) {
	conflicts := make([]string, 0)
	for _, name := range selection.Installs {
		if requiredUninstalls[name] > 0 {
			conflicts = append(conflicts, name)
		}
	}
	if len(conflicts) == 0 {
		return selection, nil
	}
	selection.Installs = withoutItems(selection.Installs, conflicts)
	if err := saveServiceLocalManifest(cfg, selection); err != nil {
		return manifest.Item{}, fmt.Errorf("remove selections overridden by administrator uninstall policy: %w", err)
	}
	return selection, nil
}

func withoutItems(existing, removed []string) []string {
	filtered := make([]string, 0, len(existing))
	for _, item := range existing {
		if !slices.Contains(removed, item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
