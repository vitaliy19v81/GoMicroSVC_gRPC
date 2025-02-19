package kafka_services

import (
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"net"
	"strconv"
)

// CreateKafkaTopic проверяет существование и создаёт топик, если его нет
func CreateKafkaTopic(brokers []string, topic string, numPartitions, replicationFactor int) error {
	controllerConn, err := getKafkaController(brokers)
	if err != nil {
		return fmt.Errorf("ошибка получения контроллера Kafka: %v", err)
	}
	defer controllerConn.Close()

	exists, err := topicExists(controllerConn, topic)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования топика: %v", err)
	}
	if exists {
		log.Printf("Топик %s уже существует", topic)
		return nil
	}

	if err := createTopic(controllerConn, topic, numPartitions, replicationFactor); err != nil {
		return fmt.Errorf("ошибка создания топика: %v", err)
	}
	log.Printf("Топик успешно создан: %s", topic)
	return nil
}

// getKafkaController получает соединение с контроллером Kafka
func getKafkaController(brokers []string) (*kafka.Conn, error) {
	for _, broker := range brokers {
		conn, err := kafka.Dial("tcp", broker)
		if err == nil {
			controller, err := conn.Controller()
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("ошибка получения контроллера: %v", err)
			}

			controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
			conn.Close()
			if err != nil {
				return nil, fmt.Errorf("ошибка подключения к контроллеру: %v", err)
			}
			return controllerConn, nil
		}
	}
	return nil, fmt.Errorf("не удалось подключиться ни к одному из брокеров: %v", brokers)
}

// topicExists проверяет, существует ли топик в Kafka
func topicExists(conn *kafka.Conn, topic string) (bool, error) {
	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return false, err
	}
	return len(partitions) > 0, nil
}

// createTopic создаёт новый топик в Kafka
func createTopic(conn *kafka.Conn, topic string, numPartitions, replicationFactor int) error {
	topicConfig := kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	}
	return conn.CreateTopics(topicConfig)
}
