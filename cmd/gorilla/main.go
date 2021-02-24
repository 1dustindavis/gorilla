package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/config"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/manifest"
	"github.com/1dustindavis/gorilla/pkg/process"
	"github.com/1dustindavis/gorilla/pkg/report"
)

func main() {

	// Get our configuration
	cfg := config.Get()
	var err error
	// if --checkonly is NOT passed, we need to run adminCheck()
	if !cfg.CheckOnly {
		admin, err := adminCheck()
		if err != nil {
			fmt.Println("Unable to check if running as admin, got: %w", err)
			os.Exit(1)
		}
		if !admin {
			fmt.Println("Gorilla requires admnisistrative access. Please run as an administrator.")
			os.Exit(1)
		}
	}

	// If needed, create the cache directory
	err = os.MkdirAll(filepath.Clean(cfg.CachePath), 0755)
	if err != nil {
		fmt.Println("Unable to create cache directory: ", err)
		os.Exit(1)
	}

	// Create a new logger object
	gorillalog.NewLog(cfg)

	// Start creating GorillaReport
	if !cfg.CheckOnly {
		report.Start()
	}

	// Set the configuration that `download` will use
	download.SetConfig(cfg)

	// Get the manifests
	gorillalog.Info("Retrieving manifest:", cfg.Manifest)
	manifests, newCatalogs := manifest.Get(cfg)

	// If we have newCatalogs, add them to the configuration
	if newCatalogs != nil {
		cfg.Catalogs = append(cfg.Catalogs, newCatalogs...)
	}

	// Get the catalogs
	gorillalog.Info("Retrieving catalog:", cfg.Catalogs)
	catalogs := catalog.Get(cfg)

	// Process the manifests into install type groups
	gorillalog.Info("Processing manifest...")
	installs, uninstalls, updates := process.Manifests(manifests, catalogs)

	// Prepare and install
	gorillalog.Info("Processing managed installs...")
	process.Installs(installs, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and uninstall
	gorillalog.Info("Processing managed uninstalls...")
	process.Uninstalls(uninstalls, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Prepare and update
	gorillalog.Info("Processing managed updates...")
	process.Updates(updates, catalogs, cfg.URLPackages, cfg.CachePath, cfg.CheckOnly)

	// Save GorillaReport to disk
	gorillalog.Info("Saving GorillaReport.json...")
	if !cfg.CheckOnly {
		report.End()
	} else {
		report.Print()
	}

	// Run CleanUp to delete old cached items and empty directories
	gorillalog.Info("Cleaning up the cache...")
	process.CleanUp(cfg.CachePath)

	gorillalog.Info("Done!")
}

// adminCheck is borrowed from https://github.com/golang/go/issues/28804#issuecomment-438838144
func adminCheck() (bool, error) {
	// Skip the check if this is test
	if flag.Lookup("test.v") != nil {
		return false, nil
	}

	var adminSid *windows.SID

	// Although this looks scary, it is directly copied from the
	// official windows documentation. The Go API for this is a
	// direct wrap around the official C++ API.
	// See https://docs.microsoft.com/en-us/windows/desktop/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&adminSid)
	if err != nil {
		return false, fmt.Errorf("SID Error: %v", err)
	}
	defer windows.FreeSid(adminSid)
	// This appears to cast a null pointer so I'm not sure why this
	// works, but this guy says it does and it Works for Me™:
	// https://github.com/golang/go/issues/28804#issuecomment-438838144
	token := windows.Token(0)

	admin, err := token.IsMember(adminSid)
	if err != nil {
		return false, fmt.Errorf("Token Membership Error: %v", err)
	}
	return admin, nil
}
