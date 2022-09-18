package util

import "testing"

func TestBase64Enc(t *testing.T) {
	plain := "hello"
	encStr := "aGVsbG8="

	str := Base64Enc(plain)
	if str != encStr {
		t.Fatal("base64 encode failed")
	}
	str, _ = Base64Dec(encStr)
	if str != plain {
		t.Fatal("base64 decode failed")
	}

}
