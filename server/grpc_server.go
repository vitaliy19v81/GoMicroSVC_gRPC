package server

import (
	"context"
	"database/sql"
	"go_micro_gRPS/internal/database"
	"go_micro_gRPS/internal/kafka_services"
	pb "go_micro_gRPS/proto/go_micro_gRPC/proto"
	"google.golang.org/grpc"
	"log"
	"net"
)

type Server struct {
	pb.UnimplementedMessageServiceServer
	db            *sql.DB
	kafkaProducer *kafka_services.Producer
}

// SendMessage Реализация метода
func (s *Server) SendMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	// Сохранение сообщения в базе данных
	id, err := database.SaveMessage(s.db, req.Content)
	if err != nil {
		log.Printf("Error saving message to database: %v", err)
		return nil, err
	}

	// Отправка сообщения в Kafka
	message := map[string]interface{}{
		"id":      id,
		"content": req.Content,
	}
	err = s.kafkaProducer.SendMessage(ctx, "default-key", message)
	if err != nil {
		log.Printf("Error sending message to Kafka: %v", err)
		return nil, err
	}

	return &pb.MessageResponse{
		Status: "Message sent successfully",
		Id:     int32(id),
	}, nil
}

// GetProcessedMessages Реализация метода
func (s *Server) GetProcessedMessages(ctx context.Context, req *pb.EmptyRequest) (*pb.MessageStats, error) {
	count, err := database.GetProcessedMessageCount(s.db)
	if err != nil {
		log.Printf("Error getting processed message count: %v", err)
		return nil, err
	}

	return &pb.MessageStats{
		ProcessedCount: int32(count),
	}, nil
}

// Функция запуска gRPC-сервера
func StartGRPCServer(db *sql.DB, producer *kafka_services.Producer) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterMessageServiceServer(s, &Server{db: db, kafkaProducer: producer})

	log.Println("Starting gRPC Server on port 50051...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
