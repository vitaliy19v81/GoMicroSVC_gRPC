// /GoMicroSVC_gRPC/cmd/api/main.go
package main

import (
	"context"
	"errors"
	httpSwagger "github.com/swaggo/http-swagger"
	"go_micro_gRPS/config"
	_ "go_micro_gRPS/docs"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/handlers"
	"go_micro_gRPS/internal/kafka_services"
	"go_micro_gRPS/server"
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

	// Вызов функции создания топика
	err = kafka_services.CreateKafkaTopic(brokers, topic, 1, 1) // brokers[0]
	if err != nil {
		log.Fatalf("Ошибка создания топика: %v\n", err)
	}

	groupID := "consumer_group_1"
	// Создаём consumer
	consumer := kafka_services.NewKafkaConsumer(brokers, topic, groupID)
	defer consumer.Close()

	// Запускаем чтение сообщений в отдельной горутине
	go func() {
		consumer.ReadMessages(ctx)
	}()

	// Инициализация Kafka producer
	kafkaProducer := kafka_services.NewKafkaProducer(ctx, brokers, topic)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Printf("Ошибка закрытия Kafka producer: %v", err)
		}
	}()

	// Запуск gRPC-сервера
	go server.StartGRPCServer(db, kafkaProducer)

	// Ручка для Swagger UI
	// export PATH=$PATH:$(go env GOPATH)/bin
	// swag init -g cmd/api/main.go -o docs
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	// curl -X POST http://localhost:8080/api/messages -d '{"content": "Hello, World!"}' -H "Content-Type: application/json"
	http.HandleFunc("/api/messages", handlers.PostMessageHTTPHandler(kafkaProducer, db))
	// {"id":"2","status":"message sent successfully"}

	// curl http://localhost:8080/api/stats
	http.HandleFunc("/api/stats", handlers.GetStatsHTTPHandler(db))
	// {"processed_messages":1}

	http.HandleFunc("/api/consume", handlers.ConsumeMessagesHandler(consumer))

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

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Рабочий код
//// CreateKafkaTopic создает новый топик в Kafka
//func CreateKafkaTopic(brokers []string, topic string, numPartitions, replicationFactor int) error {
//	var conn *kafka.Conn
//	var err error
//
//	// Пытаемся подключиться к каждому брокеру из списка
//	for _, brokerAddress := range brokers {
//		conn, err = kafka.Dial("tcp", brokerAddress)
//		if err == nil {
//			log.Printf("Успешное подключение к брокеру Kafka: %s\n", brokerAddress)
//			defer func() {
//				if err := conn.Close(); err != nil {
//					log.Printf("Ошибка закрытия соединения с брокером: %v\n", err)
//				}
//			}()
//			break
//		} else {
//			log.Printf("Не удалось подключиться к брокеру Kafka: %s, пробуем следующий", brokerAddress)
//		}
//	}
//
//	// Если не удалось подключиться ни к одному брокеру
//	if conn == nil {
//		return fmt.Errorf("не удалось подключиться ни к одному из брокеров Kafka: %v", brokers)
//	}
//
//	// Получаем контроллер кластера
//	controller, err := conn.Controller()
//	if err != nil {
//		return fmt.Errorf("ошибка получения контроллера Kafka: %v", err)
//	}
//	log.Printf("Контроллер Kafka: %s:%d\n", controller.Host, controller.Port)
//
//	// Подключаемся к контроллеру для создания топика
//	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
//	if err != nil {
//		return fmt.Errorf("ошибка подключения к контроллеру Kafka: %v", err)
//	}
//	defer func() {
//		if err := controllerConn.Close(); err != nil {
//			log.Printf("Ошибка закрытия соединения с контроллером: %v\n", err)
//		}
//	}()
//
//	// Конфигурация для создания нового топика
//	topicConfigs := []kafka.TopicConfig{
//		{
//			Topic:             topic,
//			NumPartitions:     numPartitions,
//			ReplicationFactor: replicationFactor,
//		},
//	}
//
//	// Создаем топик
//	err = controllerConn.CreateTopics(topicConfigs...)
//	if err != nil {
//		return fmt.Errorf("ошибка создания топика: %v", err)
//	}
//
//	log.Printf("Топик успешно создан: %s\n", topic)
//	return nil
//
//}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Настройка маршрутов

//// curl -X POST http://localhost:8080/messages -d '{"content": "Hello, World!"}' -H "Content-Type: application/json"
//http.HandleFunc("/messages", handlers.PostMessageHandler(kafkaProducer, db))
//// {"id":"2","status":"message sent successfully"}
//
//// curl http://localhost:8080/stats
//http.HandleFunc("/stats", handlers.GetStatsHandler(db))
//// {"processed_messages":1}

//// CreateKafkaTopic Функция для создания топика в Kafka
//func CreateKafkaTopic(brokerAddress, topic string, numPartitions, replicationFactor int) error {
//	log.Printf("Подключение к брокеру Kafka: %s\n", brokerAddress)
//
//	// Устанавливаем соединение с брокером Kafka
//	conn, err := kafka.Dial("tcp", brokerAddress)
//	if err != nil {
//		return fmt.Errorf("ошибка подключения к Kafka: %v", err)
//	}
//	defer func() {
//		if err := conn.Close(); err != nil {
//			log.Printf("Ошибка закрытия соединения с брокером: %v\n", err)
//		}
//	}()
//
//	// Получаем информацию о контроллере кластера
//	controller, err := conn.Controller()
//	if err != nil {
//		return fmt.Errorf("ошибка получения контроллера Kafka: %v", err)
//	}
//	log.Printf("Контроллер Kafka: %s:%d\n", controller.Host, controller.Port)
//
//	// Устанавливаем соединение с контроллером
//	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
//	if err != nil {
//		return fmt.Errorf("ошибка подключения к контроллеру Kafka: %v", err)
//	}
//	defer func() {
//		if err := controllerConn.Close(); err != nil {
//			log.Printf("Ошибка закрытия соединения с контроллером: %v\n", err)
//		}
//	}()
//
//	// Конфигурация нового топика
//	topicConfigs := []kafka.TopicConfig{
//		{
//			Topic:             topic,
//			NumPartitions:     numPartitions,
//			ReplicationFactor: replicationFactor,
//		},
//	}
//
//	// Создаем топик
//	err = controllerConn.CreateTopics(topicConfigs...)
//	if err != nil {
//		return fmt.Errorf("ошибка создания топика: %v", err)
//	}
//
//	log.Printf("Топик успешно создан: %s\n", topic)
//	return nil
//}
