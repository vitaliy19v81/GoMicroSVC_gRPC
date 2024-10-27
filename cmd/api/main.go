package main

import (
	"context"
	"errors"
	"go_micro_gRPS/config"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/handlers"
	"go_micro_gRPS/internal/kafka"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.LoadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация базы данных
	db, err := database.ConnectPostgres(ctx)
	if err != nil {
		log.Fatal("Ошибка подключения к базе данных:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Ошибка закрытия базы данных: %v", err)
		}
	}()

	// Получаем параметры из переменных окружения
	brokers := []string{cfg.KafkaBrokers}
	topic := cfg.KafkaTopic

	// Инициализация Kafka producer
	kafkaProducer := kafka.NewKafkaProducer(ctx, brokers, topic)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka producer: %v", err)
		}
	}()

	// Запуск gRPC-сервера
	go startGRPCServer(db, kafkaProducer)

	// Настройка маршрутов
	http.HandleFunc("/messages", handlers.PostMessageHandler(kafkaProducer, db))
	http.HandleFunc("/stats", handlers.GetStatsHandler(db))

	// Канал для получения системных сигналов для корректного завершения работы
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запуск HTTP сервера в отдельной горутине
	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		log.Println("Запуск HTTP сервера на порту 8080...")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// Ожидаем сигнал завершения
	<-quit
	log.Println("Завершение работы сервера...")

	// Контекст для корректного завершения сервера
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	log.Println("Сервер успешно завершен.")
}
