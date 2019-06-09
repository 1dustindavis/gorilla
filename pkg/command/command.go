package command

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/1dustindavis/gorilla/pkg/catalog"
	"github.com/1dustindavis/gorilla/pkg/download"
	"github.com/1dustindavis/gorilla/pkg/gorillalog"
	"github.com/1dustindavis/gorilla/pkg/report"
	"github.com/1dustindavis/gorilla/pkg/status"
)

func RunCommand(command string, arguments []string) string {
	cmd := execCommand(command, arguments...)
	var cmdOutput string
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Error creating pipe to stdout", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	scanner := bufio.NewScanner(cmdReader)
	gorillalog.Debug("command:", command, arguments)
	go func() {
		gorillalog.Debug("Command Output:")
		gorillalog.Debug("--------------------")
		for scanner.Scan() {
			gorillalog.Debug(scanner.Text())
			cmdOutput = scanner.Text()
		}
		gorillalog.Debug("--------------------")
		wg.Done()
	}()

	err = cmd.Start()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Error running command:", err)
	}

	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		gorillalog.Warn("command:", command, arguments)
		gorillalog.Warn("Command error:", err)
	}

	return cmdOutput
}
