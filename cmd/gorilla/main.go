package main

import (
	"fmt"
	"os"

	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/service"
)

var (
	managedRunFunc         = managedRun
	runServiceFunc         = func(cfg config.Configuration) error { return service.Run(cfg, managedRunFunc) }
	sendServiceCommandFunc = service.SendCommand
	runServiceActionFunc   = service.RunAction
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
		return runServiceActionFunc(cfg, "install")
	}

	if cfg.ServiceRemove {
		return runServiceActionFunc(cfg, "remove")
	}

	if cfg.ServiceStart {
		return runServiceActionFunc(cfg, "start")
	}

	if cfg.ServiceStop {
		return runServiceActionFunc(cfg, "stop")
	}

	if cfg.ServiceCommand != "" {
		resp, err := sendServiceCommandFunc(cfg, cfg.ServiceCommand)
		if err != nil {
			return err
		}
		for _, item := range resp.Items {
			fmt.Println(item)
		}
		return nil
	}

	if cfg.ServiceMode {
		return runServiceFunc(cfg)
	}

	return managedRunFunc(cfg)
}
