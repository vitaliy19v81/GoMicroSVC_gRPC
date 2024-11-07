package main

import (
	"context"
	"log"
	"time"

	pb "go_micro_gRPS/proto/go_micro_gRPC/proto"
	"google.golang.org/grpc"
)

func main() {
	// Подключаемся к gRPC серверу
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Не удалось подключиться к серверу: %v", err)
	}
	defer conn.Close()

	client := pb.NewMessageServiceClient(conn)

	// Проверяем подключение, выполняя простую RPC вызов
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Пустой запрос, чтобы проверить доступность сервера
	req := &pb.EmptyRequest{}                      // Замените на свой пустой запрос, если требуется
	_, err = client.GetProcessedMessages(ctx, req) // Вызываем метод, который существует
	if err != nil {
		log.Fatalf("Ошибка подключения к серверу: %v", err)
	}

	log.Println("Успешное подключение к gRPC серверу!")

	// Пример отправки сообщения
	sendMessage(client, "Hello, gRPC!")
	//
	// Пример получения статистики обработанных сообщений
	getProcessedMessages(client)
}

// Функция отправки сообщения
func sendMessage(client pb.MessageServiceClient, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &pb.MessageRequest{Content: content}
	res, err := client.SendMessage(ctx, req)
	if err != nil {
		log.Fatalf("Ошибка отправки сообщения: %v", err)
	}

	log.Printf("Ответ от сервера: %s, ID: %d", res.Status, res.Id)
}

// Функция получения статистики
func getProcessedMessages(client pb.MessageServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := &pb.EmptyRequest{}
	res, err := client.GetProcessedMessages(ctx, req)
	if err != nil {
		log.Fatalf("Ошибка получения статистики: %v", err)
	}

	log.Printf("Количество обработанных сообщений: %d", res.ProcessedCount)
}
