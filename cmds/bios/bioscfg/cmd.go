package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	var driver, op, src, cfgSrc string
	var dryRun bool
	flag.StringVar(&driver, "driver", "", "Driver to use for BIOS configuration. One of dell hp lenovo none dell-legacy supermicro")
	flag.StringVar(&op, "operation", "get", "Operation to perform, one of: get test apply export")
	flag.StringVar(&src, "source", "", "Source config file to read from for testing.  Can be left blank to use the current system config.  Must be in the native tooling format for the driver (racadm get --clone XML for Dell, conrep xml for HP, list for OneCli)")
	flag.StringVar(&cfgSrc, "config", "-", "Configuration to test or apply.  '-' means read from stdin.")
	flag.BoolVar(&dryRun, "dryRun", false, "Skip actually making changes when apply is the op.")
	flag.Parse()
	var cfg Configurator
	switch driver {
	case "dell-legacy":
		cfg = &dellBiosOnlyConfig{}
	case "dell":
		cfg = &dellRacadmConfig{}
	case "hp":
		cfg = &hpConfig{}
	case "lenovo":
		cfg = &lenovoConfig{}
	case "supermicro":
		cfg = &superMicroConfig{}
	case "none":
		cfg = &noneConfig{}
	default:
		log.Fatalf("Unknown driver %s", driver)
	}

	if src != "" {
		toRead, err := os.Open(src)
		if err != nil {
			log.Fatalf("Unable to open source %s: %v", src, err)
		}
		defer toRead.Close()
		cfg.Source(toRead)
	}
	exitCode := 0
	var ents map[string]Entry
	var err error
	needReboot := false
	vars := map[string]string{}
	var dec *json.Decoder
	if cfgSrc == "-" {
		dec = json.NewDecoder(os.Stdin)
	} else {
		fi, err := os.Open(cfgSrc)
		if err != nil {
			log.Fatalf("Error opening %s: %v", cfgSrc, err)
		}
		dec = json.NewDecoder(fi)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent(``, `  `)
	var res interface{}
	res = map[string]interface{}{}
	switch op {
	case "export":
		ents, err = cfg.Current()
		if err != nil {
			log.Fatalf("Error getting config: %v", err)
		}
		vals := map[string]string{}
		for k, v := range ents {
			vals[k] = v.Current
		}
		res = vals
	case "get":
		ents, err = cfg.Current()
		if err != nil {
			log.Fatalf("Error getting config: %v", err)
		}
		res = ents
	case "test":
		if err = dec.Decode(&vars); err != nil {
			log.Fatalf("Unable to parse JSON config on stdin: %v", err)
		}
		vars = cfg.FixWanted(vars)
		_, willAttempt, err := Test(cfg, vars)
		if err != nil {
			log.Fatalf("Error figuring out what would be applied: %v", err)
		}
		res = willAttempt
	case "apply":
		if err = dec.Decode(&vars); err != nil {
			log.Fatalf("Unable to parse JSON config on stdin: %v", err)
		}
		var trimmed map[string]string
		vars = cfg.FixWanted(vars)
		ents, trimmed, err = Test(cfg, vars)
		if err == nil && len(trimmed) != 0 {
			for k, v := range trimmed {
				log.Printf("Attempting to set %s to %s\n", k, v)
			}
			needReboot, err = cfg.Apply(ents, trimmed, dryRun)
			if needReboot {
				exitCode += 192
			}
		}
		if err != nil {
			exitCode += 1
		}
		res = trimmed
	default:
		log.Fatalf("Unknown op '%s'", op)
	}
	enc.Encode(res)
	if err != nil {
		log.Println(err)
	} else {
		log.Printf("Op %s succeeded", op)
	}
	os.Exit(exitCode)
}
