package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/service"
)

var (
	managedRunFunc         = managedRun
	runServiceFunc         = func(cfg config.Configuration) error { return service.Run(cfg, managedRunFunc) }
	sendServiceCommandFunc = service.SendCommand
	runServiceActionFunc   = service.RunAction
	serviceStatusFunc      = service.ServiceStatus
)

func main() {
	cfg := config.Get()
	if err := route(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func route(cfg config.Configuration) error {
	if cfg.ServiceInstall {
		if err := runServiceActionFunc(cfg, "install"); err != nil {
			return err
		}
		fmt.Println("Service installed successfully")
		return nil
	}

	if cfg.ServiceRemove {
		if err := runServiceActionFunc(cfg, "remove"); err != nil {
			return err
		}
		fmt.Println("Service removed successfully")
		return nil
	}

	if cfg.ServiceStart {
		if err := runServiceActionFunc(cfg, "start"); err != nil {
			return err
		}
		fmt.Println("Service started successfully")
		return nil
	}

	if cfg.ServiceStop {
		if err := runServiceActionFunc(cfg, "stop"); err != nil {
			return err
		}
		fmt.Println("Service stopped successfully")
		return nil
	}

	if cfg.ServiceStatus {
		status, err := serviceStatusFunc(cfg)
		if err != nil {
			return err
		}
		fmt.Println("Service status:")
		fmt.Println(status)
		return nil
	}

	if cfg.ServiceCommand != "" {
		resp, err := sendServiceCommandFunc(cfg, cfg.ServiceCommand)
		if err != nil {
			return err
		}
		action := cfg.ServiceCommand
		if i := strings.Index(action, ":"); i >= 0 {
			action = action[:i]
		}
		action = strings.ToLower(strings.TrimSpace(action))

		if len(resp.Items) > 0 {
			for _, item := range resp.Items {
				fmt.Println(item)
			}
			return nil
		}
		if resp.OperationID != "" {
			fmt.Printf("operationId: %s\n", resp.OperationID)
		}
		if resp.Message != "" {
			fmt.Println(resp.Message)
			return nil
		}
		switch action {
		case "listoptionalinstalls":
			fmt.Println("none")
		case "installitem":
			fmt.Println("InstallItem command completed successfully")
		case "removeitem":
			fmt.Println("RemoveItem command completed successfully")
		case "streamoperationstatus":
			fmt.Println("StreamOperationStatus command completed successfully")
		default:
			fmt.Println("Service command completed successfully")
		}
		return nil
	}

	if cfg.ServiceMode {
		return runServiceFunc(cfg)
	}

	return managedRunFunc(cfg)
}
