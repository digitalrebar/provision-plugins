package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type lenovoConfig struct {
	source io.Reader
	items  map[string]string
}

func (l *lenovoConfig) Source(src io.Reader) {
	l.source = src
}

func (l *lenovoConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	if l.source == nil {
		cmd := exec.Command("OneCli", "config", "save", "--file", "settings.dat")
		out := []byte{}
		out, err = cmd.CombinedOutput()
		os.Stderr.Write(out)
		if err != nil {
			return
		}
		if !cmd.ProcessState.Success() {
			err = errors.New("Error running OneCli")
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

func (l *lenovoConfig) Apply(current map[string]Entry, trimmed map[string]string) (needReboot bool, err error) {
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
	cmd := exec.Command("OneCli", "config", "restore", "--file", "apply.dat")
	out := []byte{}
	out, err = cmd.CombinedOutput()
	os.Stderr.Write(out)
	if err != nil {
		return
	}
	if !cmd.ProcessState.Success() {
		err = errors.New("Error running OneCli")
		return
	}
	needReboot = true
	return
}
