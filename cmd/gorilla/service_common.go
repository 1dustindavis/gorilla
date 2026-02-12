package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
)

type serviceCommand struct {
	Action string   `json:"action"`
	Items  []string `json:"items,omitempty"`
}

type serviceCommandResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func parseServiceCommandSpec(spec string) (serviceCommand, error) {
	var cmd serviceCommand
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
	return cmd, validateServiceCommand(cmd)
}

func validateServiceCommand(cmd serviceCommand) error {
	switch cmd.Action {
	case "run":
		if len(cmd.Items) != 0 {
			return errors.New("run action does not support items")
		}
	case "install", "uninstall", "update":
		if len(cmd.Items) == 0 {
			return fmt.Errorf("%s action requires at least one item", cmd.Action)
		}
	default:
		return fmt.Errorf("unsupported service action %q", cmd.Action)
	}

	return nil
}

func executeServiceCommand(cfg config.Configuration, cmd serviceCommand) error {
	switch cmd.Action {
	case "run":
		return run(cfg)
	case "install":
		return runSelectedItems(cfg, cmd.Items, nil, nil)
	case "uninstall":
		return runSelectedItems(cfg, nil, cmd.Items, nil)
	case "update":
		return runSelectedItems(cfg, nil, nil, cmd.Items)
	default:
		return fmt.Errorf("unsupported service action %q", cmd.Action)
	}
}

func runSelectedItems(cfg config.Configuration, installs, uninstalls, updates []string) error {
	buildMode := cfg.BuildArg || cfg.ImportArg != ""
	if !cfg.CheckOnly && !buildMode {
		admin, err := adminCheckFunc()
		if err != nil {
			return fmt.Errorf("unable to check if running as admin: %w", err)
		}
		if !admin {
			return errors.New("gorilla requires admnisistrative access. Please run as an administrator")
		}
	}

	if err := mkdirAllFunc(cfg.CachePath, 0755); err != nil {
		return fmt.Errorf("unable to create cache directory: %w", err)
	}

	gorillalog.NewLog(cfg)
	download.SetConfig(cfg)

	_, newCatalogs := manifest.Get(cfg)
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}
	catalogs := catalog.Get(cfg)

	if len(installs) > 0 {
		gorillalog.Info("Processing requested service installs...", installs)
		process.Installs(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	}
	if len(uninstalls) > 0 {
		gorillalog.Info("Processing requested service uninstalls...", uninstalls)
		process.Uninstalls(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	}
	if len(updates) > 0 {
		gorillalog.Info("Processing requested service updates...", updates)
		process.Updates(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)
	}

	process.CleanUp(cfg.CachePath)
	return nil
}
