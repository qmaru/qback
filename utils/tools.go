package utils

import (
	"github.com/qmaru/minitools/v2/data/json/gojson"
	"github.com/qmaru/minitools/v2/file"
	"github.com/qmaru/minitools/v2/hashx/blake3"
)

var (
	FileSuite   = file.New()
	Blake3Suite = blake3.New()
	JSONSuite   = gojson.New()
)
