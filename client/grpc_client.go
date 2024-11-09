package main

import (
	"context"
	"log"
	"time"

	pb "go_micro_gRPS/proto/go_micro_gRPC/proto" // Импорт сгенерированных протофайлов
	"google.golang.org/grpc"                     // Библиотека для работы с gRPC
)

func main() {
	// Устанавливаем соединение с gRPC сервером
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		// Завершаем работу, если соединение установить не удалось
		log.Fatalf("Не удалось подключиться к серверу: %v", err)
	}
	defer conn.Close() // Закрываем соединение при завершении работы программы

	// Создаем клиента gRPC для взаимодействия с сервером
	client := pb.NewMessageServiceClient(conn)

	// Проверка подключения с помощью простого вызова метода
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Создаем пустой запрос для проверки доступности сервера
	req := &pb.EmptyRequest{}
	_, err = client.GetProcessedMessages(ctx, req) // Выполняем RPC-вызов для проверки
	if err != nil {
		// Логируем и завершаем выполнение, если сервер недоступен
		log.Fatalf("Ошибка подключения к серверу: %v", err)
	}

	log.Println("Успешное подключение к gRPC серверу!")

	// Пример вызова метода отправки сообщения
	sendMessage(client, "Hello, gRPC!")

	// Пример вызова метода получения статистики обработанных сообщений
	getProcessedMessages(client)
}

// sendMessage Отправка сообщения на сервер через gRPC
func sendMessage(client pb.MessageServiceClient, content string) {
	// Устанавливаем тайм-аут для отправки сообщения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Создаем запрос с содержимым сообщения
	req := &pb.MessageRequest{Content: content}
	// Отправляем сообщение серверу через метод SendMessage
	res, err := client.SendMessage(ctx, req)
	if err != nil {
		// Логируем ошибку отправки сообщения
		log.Fatalf("Ошибка отправки сообщения: %v", err)
	}

	// Логируем успешный ответ от сервера с ID сообщения
	log.Printf("Ответ от сервера: %s, ID: %d", res.Status, res.Id)
}

// getProcessedMessages Получение статистики обработанных сообщений с сервера
func getProcessedMessages(client pb.MessageServiceClient) {
	// Устанавливаем тайм-аут для запроса
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Создаем пустой запрос для получения статистики
	req := &pb.EmptyRequest{}
	// Запрашиваем статистику обработанных сообщений у сервера через метод GetProcessedMessages
	res, err := client.GetProcessedMessages(ctx, req)
	if err != nil {
		// Логируем ошибку получения статистики
		log.Fatalf("Ошибка получения статистики: %v", err)
	}

	// Логируем количество обработанных сообщений, полученное от сервера
	log.Printf("Количество обработанных сообщений: %d", res.ProcessedCount)
}
