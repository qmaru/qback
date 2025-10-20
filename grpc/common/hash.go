package common

import (
	"os"

	"qBack/utils"
)

// CalcBlake3 计算文件 blake3-256
func CalcBlake3(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", nil
	}
	defer f.Close()

	_, err = utils.Blake3Suite.WriteFrom(f)
	if err != nil {
		return "", err
	}

	bhash := utils.Blake3Suite.SumStream()
	return bhash.ToHex(), nil
}
