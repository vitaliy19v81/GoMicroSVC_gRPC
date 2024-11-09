package server

import (
	"context"
	"database/sql"
	"go_micro_gRPS/internal/database"            // Пакет для работы с базой данных
	"go_micro_gRPS/internal/kafka_services"      // Пакет для взаимодействия с Kafka
	pb "go_micro_gRPS/proto/go_micro_gRPC/proto" // Импорт сгенерированных протофайлов
	"google.golang.org/grpc"                     // Библиотека для работы с gRPC
	"log"
	"net"
)

// Server Структура сервера, реализующая методы gRPC-сервиса
type Server struct {
	pb.UnimplementedMessageServiceServer                          // Встраивание gRPC-сервера с пустой реализацией
	db                                   *sql.DB                  // Подключение к базе данных
	kafkaProducer                        *kafka_services.Producer // Kafka-продюсер для отправки сообщений
}

// SendMessage Метод SendMessage принимает сообщение, сохраняет его в БД и отправляет в Kafka.
func (s *Server) SendMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	// Сохранение сообщения в базе данных
	id, err := database.SaveMessage(s.db, req.Content)
	if err != nil {
		// Логирование ошибки сохранения сообщения в БД
		log.Printf("Error saving message to database: %v", err)
		return nil, err
	}

	// Формирование сообщения для отправки в Kafka
	message := map[string]interface{}{
		"id":      id,
		"content": req.Content,
	}
	// Отправка сообщения в Kafka
	err = s.kafkaProducer.SendMessage(ctx, "default-key", message)
	if err != nil {
		// Логирование ошибки отправки сообщения в Kafka
		log.Printf("Error sending message to Kafka: %v", err)
		return nil, err
	}

	// Возврат ответа с подтверждением отправки
	return &pb.MessageResponse{
		Status: "Message sent successfully",
		Id:     int32(id), // Возвращаем ID сохраненного сообщения
	}, nil
}

// GetProcessedMessages Метод GetProcessedMessages возвращает количество обработанных сообщений.
func (s *Server) GetProcessedMessages(ctx context.Context, req *pb.EmptyRequest) (*pb.MessageStats, error) {
	// Получение количества обработанных сообщений из БД
	count, err := database.GetProcessedMessageCount(s.db)
	if err != nil {
		// Логирование ошибки при получении статистики
		log.Printf("Error getting processed message count: %v", err)
		return nil, err
	}

	// Возврат статистики обработанных сообщений
	return &pb.MessageStats{
		ProcessedCount: int32(count), // Количество обработанных сообщений
	}, nil
}

// StartGRPCServer Запуск gRPC-сервера и регистрация сервисов.
func StartGRPCServer(db *sql.DB, producer *kafka_services.Producer) {
	// Прослушивание порта 50051 для входящих соединений
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		// Завершаем работу с логированием, если порт недоступен
		log.Fatalf("Failed to listen: %v", err)
	}

	// Создание экземпляра gRPC-сервера
	s := grpc.NewServer()
	// Регистрация сервера сообщений, реализующего MessageServiceServer
	pb.RegisterMessageServiceServer(s, &Server{db: db, kafkaProducer: producer})

	log.Println("Starting gRPC Server on port 50051...")
	// Запуск gRPC-сервера для обслуживания входящих запросов
	if err := s.Serve(lis); err != nil {
		// Завершение работы при ошибке сервера
		log.Fatalf("Failed to serve: %v", err)
	}
}
