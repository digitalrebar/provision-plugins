package v4

var (
	RS_PREPART       = "v"
	RS_MAJOR_VERSION = "4"
	RS_MINOR_VERSION = "0"
	RS_PATCH_VERSION = "0"
	RS_EXTRA         = "-pre"
	GitHash          = "NotSet"
	BuildStamp       = "Not Set"
	RS_VERSION       = RS_PREPART + RS_MAJOR_VERSION + "." + RS_MINOR_VERSION + "." + RS_PATCH_VERSION + RS_EXTRA + "-" + GitHash
)
