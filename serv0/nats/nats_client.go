package nats_client

import (
	"encoding/json"
	"fmt"
	"log"

	"Level0/cache"
	database "Level0/db"
	"Level0/model"

	"github.com/nats-io/nats.go"
)

// NATSStreamingClient - структура для работы с NATS Streaming
type NATSStreamingClient struct {
	nc        *nats.Conn
	sub       *nats.Subscription
	db        *database.Database
	cache     *cache.Cache
	errChan   chan error
	connected bool
}

// NewNATSStreamingClient - создание нового объекта NATSStreamingClient
func NewNATSStreamingClient(natsURL string, subject string, db *database.Database, cache *cache.Cache) (*NATSStreamingClient, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	client := &NATSStreamingClient{
		nc:        nc,
		db:        db,
		cache:     cache,
		errChan:   make(chan error),
		connected: true,
	}

	client.sub, err = nc.Subscribe(subject, client.handleMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to NATS channel: %w", err)
	}

	return client, nil
}

func (c *NATSStreamingClient) handleMessage(msg *nats.Msg) {
	var order model.Order
	log.Printf("Received message: %s", string(msg.Data))

	if err := json.Unmarshal(msg.Data, &order); err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		c.errChan <- fmt.Errorf("failed to unmarshal order: %w", err)
		return
	}
	log.Printf("Order unmarshalled: %+v", order)

	if err := c.db.SaveOrder(&order); err != nil {
		log.Printf("Failed to save order to database: %v", err)
		c.errChan <- fmt.Errorf("failed to save order to database: %w", err)
		return
	}
	log.Printf("Order saved to database: %+v", order)

	// Получаем ID сохраненного заказа
	orderID, err := c.db.GetLastOrderID()
	if err != nil {
		log.Printf("Failed to get last order ID: %v", err)
		c.errChan <- fmt.Errorf("failed to get last order ID: %w", err)
		return
	}
	log.Printf("Last order ID: %d", orderID)

	// Кэшируем заказ по его ID
	if err := c.cache.SetOrder(orderID, &order); err != nil {
		log.Printf("Failed to update cache: %v", err)
		c.errChan <- fmt.Errorf("failed to update cache: %w", err)
		return
	}
	log.Printf("Order cached with ID: %d", orderID)

	c.errChan <- nil // Успешная обработка сообщения
}

// Close - закрытие объекта NATSStreamingClient
func (c *NATSStreamingClient) Close() error {
	if !c.connected {
		return nil
	}

	if err := c.sub.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe from NATS channel: %w", err)
	}

	c.nc.Close()
	c.connected = false
	return nil
}

// ErrChan - получение канала для передачи ошибок
func (c *NATSStreamingClient) ErrChan() <-chan error {
	return c.errChan
}
