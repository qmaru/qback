package common

import (
	"encoding/hex"
	"io"
	"os"

	"lukechampine.com/blake3"
)

// CalcBlake3 计算文件 blake3-256
func CalcBlake3(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", nil
	}
	defer f.Close()

	hasher := blake3.New(32, nil)
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	hashString := hex.EncodeToString(hasher.Sum(nil))
	return hashString, nil
}
