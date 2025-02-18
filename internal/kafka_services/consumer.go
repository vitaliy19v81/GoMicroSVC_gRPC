package kafka_services

import (
	"context"
	"log"
	"sync"

	"github.com/segmentio/kafka-go"
)

// Consumer представляет Kafka consumer с буфером сообщений.
type Consumer struct {
	reader   *kafka.Reader
	messages []Message
	mu       sync.Mutex
}

// Message структура для хранения сообщений.
type Message struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// NewKafkaConsumer создаёт consumer с буфером сообщений.
func NewKafkaConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			MinBytes:    10e3,
			MaxBytes:    10e6,
			StartOffset: kafka.LastOffset,
		}),
		messages: make([]Message, 0),
	}
}

// ReadMessages читает сообщения и сохраняет их в буфер.
func (c *Consumer) ReadMessages(ctx context.Context) {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			return
		}

		c.mu.Lock()
		c.messages = append(c.messages, Message{Key: string(msg.Key), Value: string(msg.Value)})
		c.mu.Unlock()

		log.Printf("Получено сообщение: Key=%s, Value=%s", msg.Key, msg.Value)
	}
}

// GetMessages возвращает буфер сообщений и очищает его.
func (c *Consumer) GetMessages() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	messages := c.messages
	c.messages = nil
	return messages
}

// Close закрывает Kafka consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
