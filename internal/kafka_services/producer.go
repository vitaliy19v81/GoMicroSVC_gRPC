// Package kafka_services /GoMicroSVC_gRPC/internal/kafka_services/producer.go
package kafka_services

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"log"
)

// Producer KafkaProducer представляет собой структуру для работы с Kafka producer
type Producer struct {
	writer *kafka.Writer
}

// NewKafkaProducer инициализирует новый Kafka producer и управляет его жизненным циклом
func NewKafkaProducer(ctx context.Context, brokers []string, topic string) *Producer {
	// Инициализация продюсера напрямую через структуру kafka_services.Writer
	producer := &Producer{
		writer: &kafka.Writer{
			Addr:        kafka.TCP(brokers...), // Адреса брокеров Kafka
			Topic:       topic,                 // Название топика
			Balancer:    &kafka.LeastBytes{},   // Балансировщик LeastBytes для равномерного распределения
			MaxAttempts: 3,                     // Максимальное количество попыток отправки
			Async:       false,                 // Синхронный режим для последовательной отправки
		},
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down Kafka producer...")
		// producer.Close()
		if err := producer.Close(); err != nil {
			log.Printf("Ошибка при закрытии Kafka producer: %v", err)
		}
	}()

	return producer
}

// SendMessage отправляет сообщение в Kafka
func (kp *Producer) SendMessage(ctx context.Context, key string, message interface{}) error {
	// Преобразуем сообщение в JSON
	msg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Ошибка сериализации сообщения: %v", err)
		return err
	}

	// Отправляем сообщение в Kafka
	err = kp.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: msg,
	})

	if err != nil {
		log.Printf("Ошибка отправки сообщения в Kafka: %v", err)
		return err
	}

	log.Println("Сообщение успешно отправлено в Kafka")
	return nil
}

// Close закрывает Kafka writer
func (kp *Producer) Close() error {
	return kp.writer.Close()
}
