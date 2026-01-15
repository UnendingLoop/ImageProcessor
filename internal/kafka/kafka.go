// Package kafka provides methods for initiating kafka-topics for the app and a kafka readiness-probing
package kafka

import (
	"context"
	"errors"
	"log"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// InitKafkaTopics - creates topics in kafka
func InitKafkaTopics(ctx context.Context, brokerAddr string, delay time.Duration, topics ...string) {
	client := &kafkago.Client{
		Addr:    kafkago.TCP(brokerAddr),
		Timeout: 10 * time.Second,
	}

	req := kafkago.CreateTopicsRequest{
		Topics: make([]kafkago.TopicConfig, 0, len(topics)),
	}

	for _, t := range topics {
		topic := kafkago.TopicConfig{
			Topic:             t,
			NumPartitions:     1,
			ReplicationFactor: 1,
		}
		req.Topics = append(req.Topics, topic)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("InitKafkaTopics canceled or timed out")
			return
		default:
		}

		resp, err := client.CreateTopics(ctx, &req)
		if err != nil {
			log.Printf("Failed to run topics creation request: %v\nWait %v before next try...", err, delay)
			time.Sleep(delay)
			continue
		}

		successT := 0
		for k, v := range resp.Errors {
			switch {
			case errors.Is(v, kafkago.TopicAlreadyExists):
				successT++
			case v == nil:
			default:
				log.Printf("Topic %q creation error: %v", k, v)
			}
		}

		if len(resp.Errors) == successT {
			log.Println("All topics created successfully!")
			return
		}
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
		time.Sleep(10 * time.Second)
	}
	log.Println("Kafka is ready!")
}
