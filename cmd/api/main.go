package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/segmentio/kafka-go"
	"go_micro_gRPS/config"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/handlers"
	"go_micro_gRPS/internal/kafka_services"
	"go_micro_gRPS/server"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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

	// Вызов функции создания топика
	err = CreateKafkaTopic(brokers[0], topic, 1, 1)
	if err != nil {
		log.Fatalf("Ошибка создания топика: %v\n", err)
	}

	// Инициализация Kafka producer
	kafkaProducer := kafka_services.NewKafkaProducer(ctx, brokers, topic)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka producer: %v", err)
		}
	}()

	// Запуск gRPC-сервера
	go server.StartGRPCServer(db, kafkaProducer)

	// Настройка маршрутов

	// curl -X POST http://localhost:8080/messages -d '{"content": "Hello, World!"}' -H "Content-Type: application/json"
	http.HandleFunc("/messages", handlers.PostMessageHandler(kafkaProducer, db))
	// {"id":"2","status":"message sent successfully"}

	// curl http://localhost:8080/stats
	http.HandleFunc("/stats", handlers.GetStatsHandler(db))
	// {"processed_messages":1}

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
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	log.Println("Сервер успешно завершен.")
}

// CreateKafkaTopic Функция для создания топика в Kafka
func CreateKafkaTopic(brokerAddress, topic string, numPartitions, replicationFactor int) error {
	log.Printf("Подключение к брокеру Kafka: %s\n", brokerAddress)

	// Устанавливаем соединение с брокером Kafka
	conn, err := kafka.Dial("tcp", brokerAddress)
	if err != nil {
		return fmt.Errorf("ошибка подключения к Kafka: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Ошибка закрытия соединения с брокером: %v\n", err)
		}
	}()

	// Получаем информацию о контроллере кластера
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("ошибка получения контроллера Kafka: %v", err)
	}
	log.Printf("Контроллер Kafka: %s:%d\n", controller.Host, controller.Port)

	// Устанавливаем соединение с контроллером
	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("ошибка подключения к контроллеру Kafka: %v", err)
	}
	defer func() {
		if err := controllerConn.Close(); err != nil {
			log.Printf("Ошибка закрытия соединения с контроллером: %v\n", err)
		}
	}()

	// Конфигурация нового топика
	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
		},
	}

	// Создаем топик
	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("ошибка создания топика: %v", err)
	}

	log.Printf("Топик успешно создан: %s\n", topic)
	return nil
}
