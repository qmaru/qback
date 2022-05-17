package common

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

// CalcMD5 计算文件 MD5
func CalcMD5(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", nil
	}
	defer f.Close()
	md5hash := md5.New()
	_, err = io.Copy(md5hash, f)
	if err != nil {
		return "", nil
	}
	md5Str := hex.EncodeToString(md5hash.Sum(nil))
	return md5Str, nil
}
