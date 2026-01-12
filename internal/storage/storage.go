package storage

import (
	"log"
	"time"

	"github.com/UnendingLoop/ImageProcessor/internal/storage/miniostorage"
	"github.com/wb-go/wbf/config"
)

func NewImgStorage(cfg *config.Config, delay time.Duration) *miniostorage.MinioImageStorage {
	success := false
	var client *miniostorage.MinioImageStorage
	var err error

	for !success {
		log.Println("Connecting to IMG-storage...")
		client, err = miniostorage.NewMinioClient(cfg)
		if err != nil {
			log.Printf("Failed to init connection to IMG-storage: %v\nNext retry in %v...", err, delay)
			time.Sleep(delay)
			continue
		}
		log.Println("Successfully connected IMG-storage!")
		success = true
	}

	return client
}
