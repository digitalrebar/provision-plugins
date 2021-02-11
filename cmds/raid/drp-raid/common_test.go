package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"testing"
)

func diff(a, b string) (string, error) {
	f1, err := ioutil.TempFile("", "clitest-diff-src")
	if err != nil {
		return "", err
	}
	defer f1.Close()
	defer os.Remove(f1.Name())
	f2, err := ioutil.TempFile("", "clitest-diff-dest")
	if err != nil {
		return "", err
	}
	defer f2.Close()
	defer os.Remove(f2.Name())
	if _, err := io.WriteString(f1, a); err != nil {
		return "", err
	}
	if _, err := io.WriteString(f2, b); err != nil {
		return "", err
	}
	cmd := exec.Command("diff", "-u", f1.Name(), f2.Name())
	res, err := cmd.CombinedOutput()
	return string(res), err
}

func ctrlrs(count int, driver string) Controllers {
	var drvr Driver
	for _, d := range allDrivers {
		if d.Name() == driver {
			drvr = d
			break
		}
	}
	if drvr == nil {
		log.Fatalf("No driver for %s", driver)
	}
	res := make(Controllers, count)
	for i := range res {
		res[i] = &Controller{
			driver:      drvr,
			ID:          strconv.Itoa(i),
			Driver:      driver,
			JBODCapable: true,
			RaidCapable: true,
			RaidLevels:  []string{"raid0", "raid1", "raid5", "raid6", "raid00", "raid10", "raid50", "raid60"},
			Volumes:     []*Volume{},
			Disks:       []*PhysicalDisk{},
		}
		res[i].PCI.Device = int64(i)
	}
	return res
}

func (c *Controller) addDisks(count int, size uint64, proto, mediaType string) *Controller {
	disks := make([]*PhysicalDisk, count)
	for i := range disks {
		disks[i] = &PhysicalDisk{
			ControllerID:       c.ID,
			ControllerDriver:   c.Driver,
			Size:               (size >> 8) << 8,
			SectorCount:        size >> 8,
			PhysicalSectorSize: 512,
			LogicalSectorSize:  512,
			Slot:               uint64(i + len(c.Disks)),
			Protocol:           proto,
			MediaType:          mediaType,
			Status:             "good",
		}
	}
	c.Disks = append(c.Disks, disks...)
	return c
}

func mc(f io.Reader) Controllers {
	res := Controllers{}
	dec := json.NewDecoder(f)
	if err := dec.Decode(&res); err != nil {
		log.Fatalf("Error decoding: %v", err)
	}
	for i := range res {
		for _, d := range allDrivers {
			if d.Name() == res[i].Driver {
				res[i].driver = d
				break
			}
		}
		if res[i].driver == nil {
			log.Fatalf("No driver for %s", res[i].Driver)
		}
	}
	return res
}

func readController(c string) Controllers {
	f, err := os.Open(c)
	if err != nil {
		log.Fatalf("Error opening controllers %s", c)
	}
	defer f.Close()
	return mc(f)
}

var (
	compileCount int
	compileFunc  string
)

func compile(t *testing.T, controllers Controllers, specs string) {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		t.Errorf("Cannot determine caller info")
		return
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		t.Errorf("Unable to determine caling function")
		return
	}
	if fn.Name() != compileFunc {
		compileFunc = fn.Name()
		compileCount = 1
	} else {
		compileCount += 1
	}
	testPath := path.Join("test-data", path.Base(compileFunc), strconv.Itoa(compileCount))
	os.MkdirAll(testPath, 0755)
	os.Remove(path.Join(testPath, "untouched"))
	specPath := path.Join(testPath, "volspecs.json")
	if err := ioutil.WriteFile(specPath, []byte(specs), 0644); err != nil {
		t.Errorf("Cannot save %s: %v", specPath, err)
		return
	}
	outPath := path.Join(testPath, "actual.json")
	out, err := os.Create(outPath)
	if err != nil {
		t.Errorf("Error creating spec dest %s: %v", outPath, err)
		return
	}
	logPath := path.Join(testPath, "actual.log")
	logger, err := os.Create(logPath)
	if err != nil {
		t.Errorf("Error creating log dest %s: %v", logPath, err)
		return
	}
	sess := newSession()
	sess.log = log.New(logger, "", 0)
	sess.out = out
	fake = true
	for i := range controllers {
		controllers[i].driver.Logger(sess.log)
	}
	sess.controllers = controllers
	sess.In(specPath).Configure(false, false)
	if sess.compiledSpecs != nil {
		sess.PrettyPrint(sess.compiledSpecs)
	}
	out.Close()
	logger.Close()
	expectOut := path.Join(testPath, "expect.json")
	expectLog := path.Join(testPath, "expect.log")
	cmd := exec.Command("diff", "-Nu", expectOut, outPath)
	res, err := cmd.CombinedOutput()
	if len(res) != 0 {
		t.Errorf("Difference between %s and %s", expectOut, outPath)
		t.Errorf("%s\n", string(res))
	} else {
		t.Logf("Compiled specs at %s matched as expected", testPath)
	}
	cmd = exec.Command("diff", "-Nu", expectLog, logPath)
	res, err = cmd.CombinedOutput()
	if len(res) != 0 {
		t.Errorf("Difference between %s and %s", expectLog, logPath)
		t.Errorf("%s\n", string(res))
	} else {
		t.Logf("Compile logs at %s matched as expected", testPath)
	}
}
