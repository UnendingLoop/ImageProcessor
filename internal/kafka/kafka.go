package kafka

import (
	"log"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// InitKafkaTopic - cerates topics in kafka
func InitKafkaTopic(brokerAddr string, topics ...string) {
	topicStructs := make([]kafkago.TopicConfig, 0, len(topics))

	for _, topic := range topics {
		topicStructs = append(topicStructs, kafkago.TopicConfig{
			Topic:             topic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		})
	}

	topicsCreated := false

	for !topicsCreated {
		conn, err := kafkago.Dial("tcp", brokerAddr)
		if err != nil {
			log.Println("Failed to dial broker:", err)
			time.Sleep(5 * time.Second)
			continue
		}
		defer func() {
			if err := conn.Close(); err != nil {
				log.Println("Failed to close reader connection to Kafka:", err)
			}
		}()

		if err := conn.CreateTopics(topicStructs...); err != nil {
			log.Println("Failed to create topics:", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Topics successfully created!")
		topicsCreated = true
	}
}

// WaitKafkaReady - timeout given to kafka-service for getting fully functional
func WaitKafkaReady(brokerAddr string) {
	for {
		conn, err := kafkago.Dial("tcp", brokerAddr)
		if err == nil {
			if errConn := conn.Close(); errConn != nil {
				log.Println("Failed to close connection after testing Kafka readyness:", errConn)
			}
			break
		}
		log.Println("Kafka not ready, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}
	time.Sleep(25 * time.Second)
}
