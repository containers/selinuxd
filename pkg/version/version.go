package version

import (
	"fmt"
	"runtime"

	"github.com/go-logr/logr"
)

var (
	buildDate string // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	version   string // the current version of the daemon
)

type Info struct {
	Version   string `json:"version,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
	Compiler  string `json:"compiler,omitempty"`
	Platform  string `json:"platform,omitempty"`
}

// AsKeyValues returns a key value slice of the info.
func (i *Info) AsKeyValues() []interface{} {
	return []interface{}{
		"version", i.Version,
		"buildDate", i.BuildDate,
		"compiler", i.Compiler,
		"platform", i.Platform,
	}
}

func getInfo() *Info {
	return &Info{
		Version:   version,
		BuildDate: buildDate,
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func printInfo(logger logr.Logger, info *Info) {
	logger.Info(
		"selinuxd information",
		info.AsKeyValues()...,
	)
}

func PrintInfoPermissive(logger logr.Logger) {
	printInfo(logger, getInfo())
}
