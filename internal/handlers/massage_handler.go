// Package handlers /GoMicroSVC_gRPC/internal/handlers/massage_handler.go
package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/kafka_services"
	"log"
	"net/http"
)

type HTTPMessage struct {
	Content string `json:"content"`
}

// MessageContent хранит только поле content.
type MessageContent struct {
	Content string `json:"content"`
}

// PostMessageHTTPHandler отправляет сообщение в Kafka и сохраняет его в БД через HTTP API
// @Summary Отправка сообщения через HTTP
// @Description Отправляет сообщение в Kafka и сохраняет его в базе данных через REST API
// @Tags messages
// @Accept json
// @Produce json
// @Param message body HTTPMessage true "Сообщение"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/messages [post]
func PostMessageHTTPHandler(producer *kafka_services.Producer, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Декодируем только содержимое сообщения
		var msg HTTPMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				log.Printf("Ошибка закрытия тела запроса: %v", err)
			}
		}()

		// Сохраняем сообщение в БД
		id, err := database.SaveMessage(db, msg.Content)
		if err != nil {
			log.Printf("Ошибка сохранения сообщения: %v", err)
			http.Error(w, "Failed to save message", http.StatusInternalServerError)
			return
		}

		// Отправляем сообщение в Kafka
		key := "default-key" // Здесь можно добавить логику генерации ключа, если нужно
		err = producer.SendMessage(context.Background(), key, map[string]interface{}{"content": msg.Content})
		if err != nil {
			log.Printf("Ошибка отправки сообщения в Kafka: %v", err)
			_ = database.UpdateMessageStatus(db, id, "failed")
			http.Error(w, "Failed to send message to Kafka", http.StatusInternalServerError)
			return
		}

		// Обновляем статус сообщения на 'processed'
		if err := database.UpdateMessageStatus(db, id, "processed"); err != nil {
			log.Printf("Ошибка обновления статуса сообщения: %v", err)
		}

		// Возвращаем успешный ответ с ID
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "message sent successfully", "id": fmt.Sprint(id)}); err != nil {
			log.Printf("Ошибка кодирования ответа: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

// GetStatsHTTPHandler возвращает статистику по обработанным сообщениям через HTTP
// @Summary Получение статистики обработанных сообщений
// @Description Возвращает количество обработанных сообщений из базы данных
// @Tags stats
// @Produce json
// @Success 200 {object} map[string]int
// @Failure 500 {object} map[string]string
// @Router /api/stats [get]
func GetStatsHTTPHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := database.GetProcessedMessageCount(db)
		if err != nil {
			http.Error(w, `{"error": "Failed to fetch stats"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]int{"processed_messages": count}); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
			return
		}
	}
}

// ConsumeMessagesHandler отдаёт сообщения, полученные из Kafka. Если нужен баланс памяти и производительности → Вариант 3 (bytes.Buffer) оптимален.
// @Summary Получение сообщений из кафки
// @Description Возвращает сообщения из кафки
// @Tags consumer
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/consume [get]
func ConsumeMessagesHandler(consumer *kafka_services.Consumer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		messages := consumer.GetMessages()
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)

		buf.WriteString("[") // Начинаем JSON-массив
		first := true

		for _, msg := range messages {
			var content MessageContent
			if err := json.Unmarshal([]byte(msg.Value), &content); err != nil {
				http.Error(w, "Ошибка декодирования сообщений", http.StatusInternalServerError)
				return
			}

			if !first {
				buf.WriteString(",") // Добавляем запятую между объектами
			}
			first = false

			if err := encoder.Encode(content); err != nil {
				http.Error(w, "Ошибка кодирования JSON", http.StatusInternalServerError)
				return
			}
		}

		buf.WriteString("]") // Закрываем JSON-массив

		// Отправляем готовый JSON одним `w.Write`
		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

//// Рабочий код Если сообщений мало → Вариант 1 (срез) удобнее, но неэффективен по памяти.
//// ConsumeMessagesHandler отдаёт сообщения, полученные из Kafka.
//// @Summary Получение сообщений из кафки
//// @Description Возвращает сообщения из кафки
//// @Tags consumer
//// @Produce json
//// @Success 200 {object} map[string]string
//// @Failure 500 {object} map[string]string
//// @Router /api/consume [get]
//func ConsumeMessagesHandler(consumer *kafka_services.Consumer) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		messages := consumer.GetMessages()
//		var parsedMessages []MessageContent
//
//		for _, msg := range messages {
//			var content MessageContent
//			if err := json.Unmarshal([]byte(msg.Value), &content); err != nil {
//				http.Error(w, "Ошибка декодирования сообщений", http.StatusInternalServerError)
//				return
//			}
//			parsedMessages = append(parsedMessages, content)
//		}
//
//		w.Header().Set("Content-Type", "application/json")
//		if err := json.NewEncoder(w).Encode(parsedMessages); err != nil {
//			http.Error(w, "Ошибка кодирования JSON", http.StatusInternalServerError)
//			return
//		}
//	}
//}
//
//// Рабочий код Если сообщений много → Вариант 2 (потоковый Flush()) предпочтительнее.
//// ConsumeMessagesHandler отдаёт сообщения, полученные из Kafka.
//// @Summary Получение сообщений из Kafka
//// @Description Возвращает сообщения из Kafka в JSON-формате в виде потока
//// @Tags consumer
//// @Produce json
//// @Success 200 {object} []MessageContent
//// @Failure 500 {object} map[string]string
//// @Router /api/consume [get]
//func ConsumeMessagesHandler(consumer *kafka_services.Consumer) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		messages := consumer.GetMessages()
//
//		w.Header().Set("Content-Type", "application/json")
//		w.Write([]byte("[")) // Начало JSON-массива
//		first := true
//
//		for _, msg := range messages {
//			var content MessageContent
//			if err := json.Unmarshal([]byte(msg.Value), &content); err != nil {
//				http.Error(w, "Ошибка декодирования сообщений", http.StatusInternalServerError)
//				return
//			}
//
//			if !first {
//				w.Write([]byte(",")) // Разделитель между объектами
//			}
//			first = false
//
//			json.NewEncoder(w).Encode(content)
//			w.(http.Flusher).Flush() // Отправляем данные немедленно
//		}
//
//		w.Write([]byte("]")) // Закрытие JSON-массива
//	}
//}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

//// Рабочий код дубль
//
//// PostMessageHandler отправляет сообщение в Kafka и сохраняет его в БД
//func PostMessageHandler(producer *kafka_services.Producer, db *sql.DB) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		// Декодируем тело запроса в переменную
//		var message map[string]interface{}
//		err := json.NewDecoder(r.Body).Decode(&message)
//		if err != nil {
//			http.Error(w, "Invalid request payload", http.StatusBadRequest)
//			return
//		}
//		//defer r.Body.Close() // Закрываем тело запроса
//		defer func() {
//			if err := r.Body.Close(); err != nil {
//				log.Printf("Ошибка закрытия тела запроса: %v", err)
//			}
//		}()
//
//		// Получаем ключ сообщения из запроса или генерируем
//		key, ok := message["key"].(string)
//		if !ok {
//			key = "default-key" // Если ключ не предоставлен, используем дефолтное значение
//		}
//
//		// Сохраняем сообщение в БД
//		id, err := database.SaveMessage(db, message["content"].(string))
//		if err != nil {
//			log.Printf("Ошибка сохранения сообщения: %v", err)
//			http.Error(w, "Failed to save message", http.StatusInternalServerError)
//			return
//		}
//
//		// Отправляем сообщение в Kafka
//		err = producer.SendMessage(context.Background(), key, message)
//		if err != nil {
//			log.Printf("Error sending message to Kafka: %v", err)
//			// Обновляем статус сообщения на 'failed' при ошибке отправки
//			_ = database.UpdateMessageStatus(db, id, "failed")
//			http.Error(w, "Failed to send message to Kafka", http.StatusInternalServerError)
//			return
//		}
//
//		// Обновляем статус сообщения на 'processed'
//		err = database.UpdateMessageStatus(db, id, "processed")
//		if err != nil {
//			log.Printf("Ошибка обновления статуса сообщения: %v", err)
//		}
//
//		// Возвращаем успешный ответ
//		w.WriteHeader(http.StatusOK)
//		if err := json.NewEncoder(w).Encode(map[string]string{"status": "message sent successfully", "id": fmt.Sprint(id)}); err != nil {
//			log.Printf("Error encoding JSON response: %v", err)
//			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
//			return
//		}
//	}
//}
//
//// GetStatsHandler возвращает статистику по обработанным сообщениям
//func GetStatsHandler(db *sql.DB) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		count, err := database.GetProcessedMessageCount(db)
//		if err != nil {
//			http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
//			return
//		}
//		w.Header().Set("Content-Type", "application/json")
//		if err := json.NewEncoder(w).Encode(map[string]int{"processed_messages": count}); err != nil {
//			log.Printf("Error encoding JSON response: %v", err)
//			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
//			return
//		}
//	}
//}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
