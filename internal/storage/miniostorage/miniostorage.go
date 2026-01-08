package miniostorage

import (
	"context"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wb-go/wbf/config"
)

type MinioImageStorage struct {
	bucket string
	client *minio.Client
}

func NewMinioClient(cfg *config.Config) (*MinioImageStorage, error) {
	bucket := cfg.GetString("BUCKET_NAME")

	if bucket == "" {
		bucket = "default"
		log.Printf("Bucket name is empty. Using default value %q...", bucket)

	}

	user := cfg.GetString("MINIO_USER")
	pass := cfg.GetString("MINIO_PASS")

	// подключаемся к минио - создаем клиента
	strg, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4(user, pass, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}

	// создаем бакет если его нет
	if err := ensureBucket(context.Background(), strg, bucket); err != nil {
		log.Printf("Failed to create bucket in MinIO:", err)
		return nil, err
	}

	return &MinioImageStorage{bucket: bucket, client: strg}, nil
}

func (s *MinioImageStorage) Put(ctx context.Context, key string, size int64, contentType string, r io.Reader) error {
	if _, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return err
	}

	return nil
}

func (s *MinioImageStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *MinioImageStorage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	res, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}

	resStat, err := res.Stat()
	if err != nil {
		return nil, "", err
	}

	return res, resStat.ContentType, nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}
