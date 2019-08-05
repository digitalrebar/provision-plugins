package main

import (
	"flag"
	"fmt"
	"os"
)

var debug DebugLevel

func main() {
	filePtr := flag.String("file", "config.yaml", "The system configuration file")
	dumpPtr := flag.Bool("dump", false, "Dump config objects")
	debugPtr := flag.Int("debug", 0, "Debug level (number bitfield)")
	inventoryPtr := flag.Bool("inventory", false, "Dump Inventory")

	flag.Parse()

	debug = DebugLevel(*debugPtr)

	c := &Config{}
	if e := c.ScanSystem(); e != nil {
		fmt.Printf("Error: scanning %v\n", e)
		os.Exit(1)
	}
	if *dumpPtr || *inventoryPtr {
		c.Dump()
	}
	if *inventoryPtr {
		os.Exit(0)
	}

	ic := &Config{}
	if e := ic.ReadConfig(*filePtr); e != nil {
		fmt.Printf("Error: parsing config file: %s :%v\n", *filePtr, e)
		os.Exit(1)
	}
	if *dumpPtr {
		ic.Dump()
	}

	rs, err := c.Apply(ic)
	for rs == ResultRescan {
		if e := c.ScanSystem(); e != nil {
			fmt.Printf("Error: scanning %v\n", e)
			os.Exit(1)
		}
		if *dumpPtr {
			c.Dump()
		}
		rs, err = c.Apply(ic)
	}
	if err != nil {
		fmt.Printf("Error Applying: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
