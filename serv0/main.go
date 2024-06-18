package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"Level0/cache"
	database "Level0/db"
	nats_client "Level0/nats"
	"Level0/server"
)

func main() {
	var (
		natsURL  = flag.String("nats-url", "nats://localhost:4222", "NATS URL")
		subject  = flag.String("subject", "subject_orders", "NATS Subject")
		dbURL    = flag.String("db-url", "postgres://postgres:Tt1193917@localhost:5432/postgres?sslmode=disable", "Database URL")
		httpPort = flag.Int("http-port", 8080, "HTTP port")
		cacheTTL = flag.Duration("cache-ttl", 10*time.Second, "Cache TTL")
	)

	flag.Parse()

	// Подключение к базе данных
	db, err := database.NewDatabase(*dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Создание кэша
	cache := cache.NewCache(db, *cacheTTL)
	defer cache.Close()

	// Создание клиента NATS
	client, err := nats_client.NewNATSStreamingClient(*natsURL, *subject, db, cache)
	if err != nil {
		log.Fatalf("failed to create NATS client: %v", err)
	}
	defer client.Close()

	// Запуск HTTP-сервера
	srv := server.NewServer(cache)
	go func() {
		if err := srv.Start(*httpPort); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Обработка ошибок NATS
	go func() {
		for err := range client.ErrChan() {
			if err != nil {
				log.Printf("NATS error: %v", err)
			}
		}
	}()

	// Обработка сигналов прерывания
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	fmt.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Printf("failed to stop server: %v", err)
	}

	if err := client.Close(); err != nil {
		log.Printf("failed to close NATS client: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("failed to close database: %v", err)
	}

	fmt.Println("Shutdown complete.")
}
