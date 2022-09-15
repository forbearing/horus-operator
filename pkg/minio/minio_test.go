package minio

import "testing"

func TestInit(t *testing.T) {
	Init()
}

func TestMakeBucket(t *testing.T) {
	bucketName := "restic"
	MakeBucket(bucketName, "us-east-1")
}
