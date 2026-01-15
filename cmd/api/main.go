// Package main (in api-subfolder) provides launch of the whole application except worker
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UnendingLoop/ImageProcessor/internal/kafka"
	"github.com/UnendingLoop/ImageProcessor/internal/repository"
	"github.com/UnendingLoop/ImageProcessor/internal/service"
	"github.com/UnendingLoop/ImageProcessor/internal/storage"
	"github.com/UnendingLoop/ImageProcessor/internal/transport"
	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/ginext"
	wbfkafka "github.com/wb-go/wbf/kafka"
	"github.com/wb-go/wbf/zlog"
)

func main() {
	// инициализировать конфиг/ считать энвы
	appConfig := config.New()
	appConfig.EnableEnv("")
	if err := appConfig.LoadEnvFiles("./.env"); err != nil {
		log.Fatalf("Failed to load envs: %s\nExiting app...", err)
	}

	// стартуем логгер
	zlog.InitConsole()
	err := zlog.SetLevel("info")
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	// готовим заранее слушатель прерываний - контекст для всего приложения
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// подключитсья к базе
	dbConn := repository.ConnectWithRetries(appConfig, 5, 10*time.Second)
	// накатываем миграцию
	repository.MigrateWithRetries(dbConn.Master, "./migrations", 10, 15*time.Second)

	// подключиться к хранилищу
	strg := storage.NewImgStorage(appConfig, 10*time.Second)
	// создаем экземпляр репо
	repo := repository.NewPostgresImageRepo(dbConn)

	// ждем пока кафка раздуплится
	broker := appConfig.GetString("KAFKA_BROKER")
	kafka.WaitKafkaReady(broker)
	// подключиться к кафке как продюсер
	topic := appConfig.GetString("KAFKA_TOPIC")
	kafka.InitKafkaTopics(ctx, broker, 10*time.Second, topic)
	pub := wbfkafka.NewProducer([]string{broker}, topic)

	// создаем экземпляр сервиса
	var svc ImageAPIService = service.NewImageService(appConfig, repo, pub, strg)
	// cоздаем экземпляр хендлера HTTP
	handlers := transport.NewImageHandler(svc)
	// сетапим сервер
	mode := appConfig.GetString("GIN_MODE")
	engine := ginext.New(mode)

	engine.GET("/ping", handlers.SimplePinger)
	engine.POST("/images/upload", handlers.Create) // создание
	engine.GET("/images/:id", handlers.LoadResult) // загрузка результата
	engine.GET("/images", handlers.GetAllImages)   // получение списка картинок с пагинацией и сортировкой
	engine.DELETE("/images/:id", handlers.Delete)  // удаление
	engine.Static("/web", "./internal/web")

	srv := &http.Server{
		Addr:    ":" + appConfig.GetString("APP_PORT"),
		Handler: engine,
	}

	// Server launch
	go func() {
		log.Printf("Server running on http://localhost%s\n", srv.Addr)
		err := srv.ListenAndServe()
		if err != nil {
			switch {
			case errors.Is(err, http.ErrServerClosed):
				log.Println("Server gracefully stopping...")
			default:
				log.Printf("Server stopped: %v", err)
				stop()
			}
		}
	}()

	// запускаем фонового воркера для отслеживания подвисших задач
	go recoveryLoop(ctx, svc)

	// ждем отмены контекста для запуска грейсфул закрытия соединений бд и кафки
	<-ctx.Done()

	shutdown(pub, dbConn)
	log.Println("Exiting worker...")
}

func recoveryLoop(ctx context.Context, svc ImageAPIService) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovery loop crashed:", r)
		}
	}()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			svc.ReviveOrphans(context.Background(), 20)
		}
	}
}

func shutdown(cons *wbfkafka.Producer, dbConn *dbpg.DB) {
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
