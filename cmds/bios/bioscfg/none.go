package main

import "io"

type noneConfig struct{}

func (n *noneConfig) Source(r io.Reader) {
	return
}

func (n *noneConfig) Current() (res map[string]Entry, err error) {
	res = map[string]Entry{}
	return
}

func (n *noneConfig) FixWanted(wanted map[string]string) map[string]string {
	return wanted
}

func (n *noneConfig) Apply(current map[string]Entry, trimmed map[string]string) (needReboot bool, err error) {
	return
}
