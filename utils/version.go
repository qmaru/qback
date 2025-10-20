package utils

import (
	"fmt"
)

const (
	DateVer   string = "COMMIT_DATE"
	CommitVer string = "COMMIT_VERSION"
	GoVer     string = "COMMIT_GOVER"
)

var VERSION string = getVersion()

func getVersion() string {
	if CommitVer == "COMMIT_VERSION" {
		return fmt.Sprintf("%s (%s)", DateVer, GoVer)
	}
	return fmt.Sprintf("%s (git-%s) (%s)", DateVer, CommitVer, GoVer)
}
