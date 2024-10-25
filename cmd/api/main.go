package main

import (
	"context"
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
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	// Инициализация базы данных
	db, err := database.ConnectPostgres()
	if err != nil {
		log.Fatal("Ошибка подключения к базе данных:", err)
	}
	defer db.Close()

	// Инициализация Kafka producer
	brokers := []string{"localhost:9092"}
	topic := "messages_topic_2"
	kafkaProducer := kafka.NewKafkaProducer(brokers, topic)
	defer kafkaProducer.Close()

	// Настройка маршрутов
	http.HandleFunc("/messages", handlers.PostMessageHandler(kafkaProducer, db)) // Передаем db
	http.HandleFunc("/stats", handlers.GetStatsHandler(db))

	// Канал для получения системных сигналов для корректного завершения работы
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запуск HTTP сервера в отдельной горутине
	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil, // по умолчанию используется стандартный http.DefaultServeMux
	}

	go func() {
		log.Println("Запуск HTTP сервера на порту 8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// Ожидаем системного сигнала завершения (Ctrl+C или SIGTERM)
	<-quit
	log.Println("Завершение работы сервера...")

	// Контекст с таймаутом для корректного завершения сервера
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	log.Println("Сервер успешно завершен.")
}
