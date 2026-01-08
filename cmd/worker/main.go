package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UnendingLoop/ImageProcessor/internal/kafka"
	"github.com/UnendingLoop/ImageProcessor/internal/repository"
	"github.com/UnendingLoop/ImageProcessor/internal/service"
	"github.com/UnendingLoop/ImageProcessor/internal/storage"
	"github.com/UnendingLoop/ImageProcessor/internal/worker"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/dbpg"
	wbfkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/retry"
)

func main() {
	// инициализировать конфиг/ считать энвы
	appConfig := config.New()
	appConfig.EnableEnv("")
	if err := appConfig.LoadEnvFiles("./.env"); err != nil {
		log.Fatalf("Failed to load envs: %s\nExiting app...", err)
	}

	// подключитсья к базе
	dbConn := repository.ConnectWithRetries(appConfig, 5, 10*time.Second)
	// подкллючиться к хранилищу
	strg := storage.NewImgStorage(appConfig)
	// создаем экземпляр репо
	repo := repository.NewPostgresImageRepo(dbConn)
	// создаем экземпляр сервиса
	var svc ImageWorkerService = service.NewImageService(repo, NoopPublisher{}, nil)

	// ждем пока кафка раздуплится
	broker := appConfig.GetString("KAFKA_BROKER")
	kafka.WaitKafkaReady(broker)
	// подключиться к кафке как читатель
	queue := make(chan kafkago.Message)
	retryStrategy := retry.Strategy{
		Attempts: 5,
		Delay:    2 * time.Second,
		Backoff:  1.5,
	}
	topic := appConfig.GetString("KAFKA_TOPIC")
	groupID := appConfig.GetString("KAFKA_GROUPID")
	cons := wbfkafka.NewConsumer([]string{broker}, topic, groupID)

	// Listening to interruptions through context - здесь точно лучшее место для объявления слушателя?
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cons.StartConsuming(ctx, queue, retryStrategy)

	// Собираем воедино все что нужно воркеру и запускаем его
	go worker.NewWorkerInstance(strg, svc, queue, cons, appConfig.GetString("RESULT_KEY")).StartWorker(ctx) // немного напрягает что нет присвоения реальной переменной, а сразу запуск

	// Waiting for interruption to stop context to start Graceful shutdown
	<-ctx.Done()

	shutdown(cons, dbConn)
	log.Println("Exiting worker...")
}

func shutdown(cons *wbfkafka.Consumer, dbConn *dbpg.DB) {
	log.Println("Interrupt received!!! Starting shutdown sequence...")

	// Closing Kafka connection:
	if err := cons.Close(); err != nil {
		log.Println("Failed to close Kafka-reader:", err)
	}
	log.Println("Kafka-consumer connection closed.")

	// Closing DB connection
	if err := dbConn.Master.Close(); err != nil {
		log.Println("Failed to close DB-conn correctly:", err)
		return
	}
	log.Println("DBconn closed")
}
