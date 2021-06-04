package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type lenovoConfig struct {
	source io.Reader
	items  map[string]string
}

func runOneCli(args ...string) error {
	cmd := exec.Command("OneCli", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil || !cmd.ProcessState.Success() {
		if err == nil {
			err = fmt.Errorf("Error running OneCli: %s", cmd.ProcessState)
		}
	}
	return err
}

func (l *lenovoConfig) Source(src io.Reader) {
	l.source = src
}

func (l *lenovoConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if l.source == nil {
		if err != runOneCli("config", "save", "--file", "settings.dat") {
			return
		}
		var fi *os.File
		fi, err = os.Open("settings.dat")
		if err != nil {
			return
		}
		defer fi.Close()
		l.source = fi
	}
	sc := bufio.NewScanner(l.source)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.HasPrefix(parts[0], `AvagoMegaRAIDConfigurationTool`) {
			// ugh, just no.  We have RAID config stuff for that
			continue
		}
		res[parts[0]] = Entry{
			Name:    parts[0],
			Current: parts[1],
		}
	}
	return
}

func (l *lenovoConfig) FixWanted(wanted map[string]string) map[string]string {
	return wanted
}

func (l *lenovoConfig) Apply(current map[string]Entry, trimmed map[string]string, dryRun bool) (needReboot bool, err error) {
	var fi *os.File
	fi, err = os.Create("apply.dat")
	if err != nil {
		return
	}
	defer fi.Close()
	for k, v := range trimmed {
		if _, err = fmt.Fprintf(fi, "%s=%s\n", k, v); err != nil {
			return
		}
	}
	if dryRun {
		return
	}
	if err = runOneCli("config", "restore", "--file", "apply.dat"); err == nil {
		needReboot = true
		return
	}
	logs, _ := filepath.Glob("logs/OneCli-*/OneCli-config-restore-*.txt")
	for _, lName := range logs {
		fi, fe := os.Open(lName)
		if fe == nil {
			defer fi.Close()
			io.Copy(os.Stderr, fi)
		}
	}
	return
}
