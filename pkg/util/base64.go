package util

import "encoding/base64"

func Base64Enc(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func Base64Dec(str string) (string, error) {
	decStr, err := base64.StdEncoding.DecodeString(str)
	return string(decStr), err
}
