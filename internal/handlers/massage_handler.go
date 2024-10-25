package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/kafka"
	"log"
	"net/http"
)

type Message struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
}

// PostMessageHandler обрабатывает HTTP POST запросы для отправки сообщений в Kafka
func PostMessageHandler(producer *kafka.Producer, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Декодируем тело запроса в переменную
		var message map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&message)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		defer r.Body.Close() // Закрываем тело запроса

		// Получаем ключ сообщения из запроса или генерируем
		key, ok := message["key"].(string)
		if !ok {
			key = "default-key" // Если ключ не предоставлен, используем дефолтное значение
		}

		// Сохраняем сообщение в БД
		id, err := database.SaveMessage(db, message["content"].(string))
		if err != nil {
			log.Printf("Ошибка сохранения сообщения: %v", err)
			http.Error(w, "Failed to save message", http.StatusInternalServerError)
			return
		}

		// Отправляем сообщение в Kafka
		err = producer.SendMessage(context.Background(), key, message)
		if err != nil {
			log.Printf("Error sending message to Kafka: %v", err)
			// Обновляем статус сообщения на 'failed' при ошибке отправки
			_ = database.UpdateMessageStatus(db, id, "failed")
			http.Error(w, "Failed to send message to Kafka", http.StatusInternalServerError)
			return
		}

		// Обновляем статус сообщения на 'processed'
		err = database.UpdateMessageStatus(db, id, "processed")
		if err != nil {
			log.Printf("Ошибка обновления статуса сообщения: %v", err)
		}

		// Возвращаем успешный ответ
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "message sent successfully", "id": fmt.Sprint(id)}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// GetStatsHandler возвращает статистику по обработанным сообщениям
func GetStatsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := database.GetProcessedMessageCount(db)
		if err != nil {
			http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]int{"processed_messages": count}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}