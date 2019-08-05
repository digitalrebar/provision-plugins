package main

import (
	"fmt"
	"strings"
)

//
// Image defines an image to put on the system
//
type Image struct {
	//
	// Path is used to define the target of the action.
	// When used stand-alone this is an optional field that
	// represents the path in the chroot where the image will be
	// placed.
	//
	// When used as part of another object, this is overriden by
	// the container's path.
	//
	// optional: true
	//
	Path string `json:"path,omitempty"`
	//
	// URL is the path to the image
	//
	// required: true
	//
	URL string `json:"url"`
	//
	// Type of image
	//
	// required: true
	//
	Type string `json:"type"`

	// Used by calling containers to indicate restricted types.
	// Should be config validated.
	rawOnly bool
	tarOnly bool
}

//
// Images defines an array of Image
//
type Images []*Image

//
// NewImage creates a new Image
//
func NewImage(url, itype string) *Image {
	return &Image{URL: url, Type: itype, Path: "/"}
}

var extractors = map[string]string{
	"dd-tgz": "|tar -xOzf -",
	"dd-txz": "|tar -xOJf -",
	"dd-tbz": "|tar -xOjf -",
	"dd-tar": "|tar -xOf -",
	"dd-bz2": "|bzcat",
	"dd-gz":  "|zcat",
	"dd-xz":  "|xzcat",
	"dd-raw": "",
	"tgz":    "|tar -Sxpzf - --numeric-owner --xattrs --xattrs-include=* -C",
	"txz":    "|tar -SxpJf - --numeric-owner --xattrs --xattrs-include=* -C",
	"tbz":    "|tar -Sxpjf - --numeric-owner --xattrs --xattrs-include=* -C",
	"tar":    "|tar -Sxpf - --numeric-owner --xattrs --xattrs-include=* -C",
}

//
// Validate validates an Image object.
//
func (i *Image) Validate() error {
	out := []string{}
	if i.Path != "" && !strings.HasPrefix(i.Path, "/") {
		out = append(out, "Path must start with /")
	}

	_, ok := extractors[i.Type]
	if !ok {
		out = append(out, fmt.Sprintf("Unknown image type for container: %s", i.Type))
	}

	if i.rawOnly && !strings.HasPrefix(i.Type, "dd-") {
		out = append(out, fmt.Sprintf("Bad image type for container: %s", i.Type))
	}
	if i.tarOnly && strings.HasPrefix(i.Type, "dd-") {
		out = append(out, fmt.Sprintf("Bad image type for container: %s", i.Type))
	}

	if len(out) > 0 {
		return fmt.Errorf(strings.Join(out, "\n"))
	}
	return nil
}

//
// Action puts the image in place using the type to define how.
//
// The Image action assumes that the image can be streamed.
//
func (i *Image) Action() error {
	dmsg(DbgImage, "Image Action: %s %s %s\n", i.Type, i.URL, i.Path)
	command := "bash"

	fileAccess := ""
	if strings.HasPrefix(i.URL, "http://") || strings.HasPrefix(i.URL, "https://") {
		fileAccess = fmt.Sprintf(`curl -g "%s"`, i.URL)
	} else if strings.HasPrefix(i.URL, "file://") {
		fileAccess = fmt.Sprintf(`cat "%s"`, i.URL[7:])
	} else {
		fileAccess = fmt.Sprintf(`cat "%s"`, i.URL)
	}

	extractor, _ := extractors[i.Type]
	dd := ""
	if i.rawOnly {
		dd = fmt.Sprintf(`| dd bs=4M of="%s"`, i.Path)
	} else {
		dd = i.Path
	}

	bout, err := runCommand(command, "-c", fmt.Sprintf("%s %s %s", fileAccess, extractor, dd))
	dmsg(DbgImage, "Deployed image: %s\n%v\n", string(bout), err)
	if err != nil {
		return err
	}

	// Start a rescan.
	if i.rawOnly {
		runCommand("partprobe", i.Path)
		runCommand("pvscan", "--cache", "--activate=ay")
	}

	return nil
}
