package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
)

var allDrivers = []Driver{
	&SsaCli{"ssacli", "/opt/smartstorageadmin/ssacli/bin/ssacli", 20, nil},
	&MegaCli{"storcli7", "/opt/MegaRAID/storcli7/storcli", 25, nil},
	&MegaCli{"storcli6", "/opt/MegaRAID/storcli6/storcli", 50, nil},
	&MegaCli{"megacli", "/opt/MegaRAID/MegaCli/MegaCli64", 75, nil},
	&PercCli{"perccli", "/opt/MegaRAID/perccli/perccli64", 10, nil},
	&MVCli{"mvcli", "/usr/local/bin/mvcli", 85, nil},
}

var fake = false

type session struct {
	in            io.Reader
	out           io.Writer
	log           *log.Logger
	controllers   Controllers
	inSpecs       VolSpecs
	compiledSpecs VolSpecs
	errors        bool
}

func newSession() *session {
	return &session{in: os.Stdin, out: os.Stdout, log: log.New(os.Stderr, "", 0)}
}

func (s *session) Log(w io.Writer) *session {
	s.log = log.New(w, "", 0)
	return s
}

func (s *session) PrettyPrint(val interface{}) {
	out := json.NewEncoder(s.out)
	out.SetIndent("  ", "  ")
	out.Encode(val)
}

func (s *session) HasError() bool {
	return s.errors
}

func (s *session) ExitOnError() {
	if s.HasError() {
		os.Exit(1)
	}
}

func (s *session) Errorf(f string, args ...interface{}) {
	s.errors = true
	s.log.Printf(f, args...)
}

func (s *session) In(src string) *session {
	switch src {
	case "", "-":
		s.in = os.Stdin
	default:
		var err error
		s.in, err = os.Open(src)
		if err != nil {
			s.Errorf("Error opening In %s: %v", src, err)
		}
	}
	return s
}

func (s *session) Out(dest string) *session {
	switch dest {
	case "", "-":
		s.out = os.Stdout
	default:
		var err error
		s.out, err = os.Create(dest)
		if err != nil {
			s.Errorf("Error opening Out %s: %v", dest, err)
		}
	}
	return s
}

func (s *session) Controllers(src string) *session {
	switch src {
	case "":
		drivers := []Driver{}
		controllers := Controllers([]*Controller{})
		for _, driver := range allDrivers {
			driver.Logger(s.log)
			if err := DriverInstalled(driver); err != nil {
				s.log.Println(err)
				continue
			}
			if !driver.Useable() {
				s.log.Printf("%s: not useable", driver.Name())
				continue
			}
			drivers = append(drivers, driver)
		}
		for _, driver := range drivers {
			controllers = append(controllers, driver.Controllers()...)
		}
		sort.Stable(controllers)
		// Dedupe controllers
		newControllers := Controllers{}
		var lastController *Controller
		for _, controller := range controllers {
			if lastController == nil {
				lastController = controller
				controller.idx = len(newControllers)
				newControllers = append(newControllers, controller)
				continue
			}
			if controller.PCI == lastController.PCI {
				continue
			}
			lastController = controller
			controller.idx = len(newControllers)
			newControllers = append(newControllers, controller)
		}
		s.controllers = newControllers
		return s
	default:
		data, err := ioutil.ReadFile(src)
		if err != nil {
			s.Errorf("Error opening controller json file: %s: %v", src, err)
			return s
		}
		if err := json.Unmarshal(data, &s.controllers); err != nil {
			s.Errorf("Error parsing controller json file: %s: %v", src, err)
			return s
		}
		for i, c := range s.controllers {
			c.idx = i
			for _, d := range allDrivers {
				if c.Driver == d.Name() {
					c.driver = d
					break
				}
			}
		}
	}
	return s
}

func (s *session) CurrentSpecs(specific bool) VolSpecs {
	if fake {
		return VolSpecs{}
	}
	return s.controllers.ToVolSpecs(specific)
}

func (s *session) WantedSpecs() *session {
	if s.inSpecs != nil {
		return s
	}
	if s.in == nil {
		s.Errorf("No source to read specs from")
		return s
	}
	inSpecs := VolSpecs{}
	dec := json.NewDecoder(s.in)
	if err := dec.Decode(&inSpecs); err != nil {
		s.Errorf("Unable to decode JSON on stdin: %v", err)
		return s
	}
	s.inSpecs = inSpecs
	return s
}

func (s *session) Compile() *session {
	if s.compiledSpecs != nil {
		return s
	}
	if s.inSpecs == nil {
		s.WantedSpecs()
	}
	if len(s.inSpecs) == 0 {
		s.log.Printf("No volspecs present to compile from")
		return s
	}
	outSpecs, err := s.inSpecs.Compile(s, s.controllers)
	if err != nil {
		s.Errorf("Error compiling volume specs: %v", err)
		return s
	}
	s.compiledSpecs = outSpecs
	s.log.Printf("Started with %d specs, compiled to %d", len(s.inSpecs), len(s.compiledSpecs))
	return s
}

func toAdd(same map[string]struct{}, specs VolSpecs) VolSpecs {
	res := VolSpecs{}
	for _, spec := range specs {
		if _, ok := same[spec.Key()]; ok {
			continue
		}
		res = append(res, spec)
	}
	return res
}

func (s *session) Diff() (map[string]VolSpecs, error) {
	if s.compiledSpecs == nil {
		return nil, fmt.Errorf("Must Compile wanted specs first")
	}
	rm := VolSpecs{}
	current := s.CurrentSpecs(true).ByKey()
	wanted := s.compiledSpecs.ByKey()
	same := map[string]struct{}{}
	for k := range wanted {
		if _, ok := current[k]; ok {
			same[k] = struct{}{}
		}
	}
	currents := VolSpecs{}
	for k, v := range current {
		if _, ok := same[k]; !ok && !v.Fake {
			rm = append(rm, v)
		} else {
			currents = append(currents, v)
		}
	}
	sort.Stable(rm)
	return map[string]VolSpecs{
		"current": currents,
		"add":     toAdd(same, s.compiledSpecs),
		"rm":      rm,
	}, nil
}

func (s *session) Clear() {
	for _, c := range s.controllers {
		if err := c.Clear(); err != nil {
			s.Errorf("Error clearing controller configs: %v", err)
		}
	}
}

func (s *session) Encrypt(key, password string) {
	for _, c := range s.controllers {
		if err := c.Encrypt(key, password); err != nil {
			s.Errorf("Error encrypting controller: %v", err)
		}
	}
}

func (s *session) Configure(doAppend, force bool) {
	s.WantedSpecs()
	if s.HasError() {
		return
	}
	if doAppend {
		s.inSpecs = append(s.CurrentSpecs(true), s.inSpecs...)
	}
	s.Compile()
	if s.HasError() {
		return
	}
	cmp, _ := s.Diff()
	if len(cmp[`rm`]) != 0 {
		s.Errorf("Cannot remove volumes using -configure")
		return
	}
	if len(cmp[`add`]) == 0 {
		s.log.Printf("All volumes already present, nothing to to")
		return
	}
	startingIndex := len(cmp[`current`])
	failed := false
	for ii, spec := range cmp[`add`] {
		c := s.controllers[spec.Controller]
		spec.index = startingIndex + ii
		if err := c.Create(spec, force); err != nil {
			failed = true
			s.log.Printf("Error creating %s on %s:%s : %v",
				spec.RaidLevel,
				c.Driver,
				c.ID,
				err)
		} else {
			s.log.Printf("Created %s on %s:%s",
				spec.RaidLevel,
				c.Driver,
				c.ID)
		}
	}
	if !fake {
		s.Controllers("")
	}
	if failed {
		s.Errorf("Failed to create some volumes")
	}
}

func main() {
	var volspecs, config, clear, force, compile, compare, addthem, encrypt, generic bool
	var controllerFile string
	var password, key string
	flag.BoolVar(&generic, "generic", false, "Output volspecs in generic format")
	flag.BoolVar(&volspecs, "volspecs", false, "Output volspecs for all currently configured RAID volumes")
	flag.BoolVar(&config, "configure", false, "Configure volumes on raid controllers to match volspecs on stdin")
	flag.BoolVar(&compare, "compare", false, "Compare current config with passed-in volspecs")
	flag.BoolVar(&clear, "clear", false, "Clear all local and foreign configuration")
	flag.BoolVar(&force, "force", false, "Force any drives to be good when configuring or wiping")
	flag.BoolVar(&compile, "compile", false, "Compile volspecs on stdin to final ones for the controller on stdout")
	flag.BoolVar(&encrypt, "encrypt", false, "Encrypt the volumes on the controllers with the key and password. It implies clear")
	flag.StringVar(&password, "password", "", "Password for encryption")
	flag.StringVar(&key, "key", "", "Key for encryption")
	flag.BoolVar(&addthem, "append", false, "Add new volumes to existing ones")
	flag.StringVar(&controllerFile, "controller", "", "Controller json file for testing")
	flag.Parse()
	s := newSession().Controllers(controllerFile)
	s.ExitOnError()
	if compare {
		cmp, err := s.Compile().Diff()
		if err != nil {
			s.log.Fatalln(err.Error())
		}
		s.ExitOnError()
		s.PrettyPrint(cmp)
		if len(cmp[`add`]) == 0 && len(cmp[`rm`]) == 0 {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	if volspecs {
		s.PrettyPrint(s.CurrentSpecs(!generic))
		os.Exit(0)
	}
	if clear {
		s.Clear()
		s.ExitOnError()
		os.Exit(0)
	}
	if encrypt {
		s.Clear()
		s.Encrypt(key, password)
		s.ExitOnError()
		os.Exit(0)
	}
	if compile {
		s.Compile()
		s.ExitOnError()
		s.PrettyPrint(s.compiledSpecs)
		os.Exit(0)
	}
	if addthem {
		s.Configure(true, force)
	}
	if config {
		s.Configure(false, force)
	}
	s.PrettyPrint(s.controllers)
	s.ExitOnError()
	os.Exit(0)
}
