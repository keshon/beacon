package web

import "github.com/keshon/buildinfo"

func buildVersion() string {
	bi := buildinfo.Get()
	return bi.BuildTime + " " + bi.GoVersion + " (" + bi.Commit + ")"
}
