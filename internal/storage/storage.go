package storage

import (
	"log"

	"github.com/UnendingLoop/ImageProcessor/internal/storage/miniostorage"
	"github.com/wb-go/wbf/config"
)

func NewImgStorage(cfg *config.Config) *miniostorage.MinioImageStorage {
	client, err := miniostorage.NewMinioClient(cfg)
	if err != nil {
		log.Fatalf("Failed to init client connection to IMG-storage: %v", err)
	}
	return client
}
