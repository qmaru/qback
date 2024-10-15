package cmd

import (
	"fmt"
)

const (
	DateVer   string = "COMMIT_DATE"
	CommitVer string = "COMMIT_VERSION"
	GoVer     string = "COMMIT_GOVER"
)

var VERSION string = fmt.Sprintf("%s (git-%s) (%s)", DateVer, CommitVer, GoVer)
