package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var bucket = "images"
var location = "us-east-1"

func initMinio() {
	endpoint := fmt.Sprintf(
		"%s:%s",
		os.Getenv("MINIO_HOST"),
		os.Getenv("MINIO_API_PORT"),
	)
	accessKeyID := os.Getenv("MINIO_ROOT_USER")
	secretAccessKey := os.Getenv("MINIO_ROOT_PASSWORD")

	var err error

	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: strings.ToLower(os.Getenv("MINIO_SECURE_CONN")) == "true",
	})

	if err != nil {
		Logger.Fatalln(err)
	}
}

func UploadToMinIO(filePath string, name string) (minio.UploadInfo, error) {
	ctx := context.Background()
	err := minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: location})

	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, bucket)
		if errBucketExists != nil || !exists {
			Logger.Fatalln(err)
		}
	}

	contentType := "application/x-gzip"

	info, err := minioClient.FPutObject(ctx, bucket, name, filePath, minio.PutObjectOptions{ContentType: contentType})

	return info, err
}

func DeleteFromMinIO(key string) error {
	ctx := context.Background()
	return minioClient.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{ForceDelete: true})
}
