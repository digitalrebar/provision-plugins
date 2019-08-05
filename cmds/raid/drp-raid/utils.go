package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type SizeParseError error

func kv(line string, sep string) (string, string) {
	parts := strings.SplitN(line, sep, 2)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return "", ""
	default:
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
}

func sizeParser(v string) (uint64, error) {
	sizeRE := regexp.MustCompile(`([0-9.]+) *([KMGTP]?B)`)
	parts := sizeRE.FindStringSubmatch(v)
	if len(parts) < 2 {
		return 0, SizeParseError(fmt.Errorf("%s cannot be parsed as a Size", v))
	}
	f, err := strconv.ParseFloat(parts[1], 10)
	if err != nil {
		return 0, SizeParseError(err)
	}
	if len(parts) == 3 {
		switch parts[2] {
		case "PB":
			f = f * 1024
		case "TB":
			f = f * 1024
			fallthrough
		case "GB":
			f = f * 1024
			fallthrough
		case "MB":
			f = f * 1024
			fallthrough
		case "KB":
			f = f * 1024
		case "B":
		default:
			return 0, SizeParseError(fmt.Errorf("%s is not a valid size suffix", parts[2]))
		}
	}
	return uint64(f), nil
}

func mustSize(v string) uint64 {
	res, err := sizeParser(v)
	if err != nil {
		log.Panicf("err: %v", err)
	}
	return res
}

func sizeStringer(s uint64) string {
	var suffix string
	var i int
	for i, suffix = range []string{"B", "KB", "MB", "GB", "TB", "PB"} {
		mul := uint64(1) << ((uint64(i) + 1) * 10)
		if uint64(s) < mul {
			break
		}
	}
	resVal := float64(s) / float64(uint64(1)<<(uint64(i)*10))
	return fmt.Sprintf("%s %s", strconv.FormatFloat(resVal, 'f', 2, 64), suffix)
}

func partitionAt(lines []string, splitter *regexp.Regexp) (prefix []string, sections [][]string) {
	prefix = []string{}
	sections = [][]string{}
	section := []string{}
	matchStarted := false
	for _, line := range lines {
		if !splitter.MatchString(line) {
			if !matchStarted {
				prefix = append(prefix, line)
			} else {
				section = append(section, line)
			}
		} else {
			if matchStarted {
				sections = append(sections, section)
			} else {
				matchStarted = true
			}
			section = []string{line}
		}
	}
	if matchStarted {
		sections = append(sections, section)
	}
	return prefix, sections
}
