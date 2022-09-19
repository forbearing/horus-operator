package minio

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

func New(endpoint, accessKeyID, secretAccessKey string, useSSL bool) *minio.Client {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		logrus.Fatal(err)
	}
	return minioClient
}

func MakeBucket(client *minio.Client, bucketName string, location string) error {
	//endpoint := "10.250.16.21:9000"
	//accessKeyID := "minioadmin"
	//secretAccessKey := "minioadmin"
	//useSSL := false
	//client := New(endpoint, accessKeyID, secretAccessKey, useSSL)
	ctx := context.TODO()

	var err error
	if err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location}); err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := client.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			//logrus.Infof("We already own %s", bucketName)
		} else {
			logrus.Fatal(err)
			return err
		}
	} else {
		//logrus.Infof("Successfully created %s", bucketName)
	}
	return nil
}
