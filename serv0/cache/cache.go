package cache

import (
	"fmt"
	"log"
	"sync"
	"time"

	database "Level0/db"
	"Level0/model"
)

type Cache struct {
	data   map[int]*model.Order
	mutex  sync.Mutex
	db     *database.Database
	ttl    time.Duration
	ticker *time.Ticker
}

func NewCache(db *database.Database, ttl time.Duration) *Cache {
	cache := &Cache{
		data:   make(map[int]*model.Order),
		mutex:  sync.Mutex{},
		db:     db,
		ttl:    ttl,
		ticker: time.NewTicker(ttl / 2),
	}

	go cache.syncFromDatabase()
	return cache
}

func (c *Cache) GetOrder(orderID int) (*model.Order, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	order, ok := c.data[orderID]
	if !ok {
		// Если заказ не найден в кэше, получаем его из базы данных
		order, err := c.db.GetOrderByID(orderID)
		if err != nil {
			return nil, fmt.Errorf("order not found: %w", err)
		}

		// Сохраняем заказ в кэше
		c.data[orderID] = order
		return order, nil
	}

	return order, nil
}

func (c *Cache) SetOrder(orderID int, order *model.Order) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data[orderID] = order
	return nil
}

func (c *Cache) syncFromDatabase() {
	for range c.ticker.C {
		rows, err := c.db.GetDB().Query("SELECT order_id FROM \"Order\"")
		if err != nil {
			log.Printf("failed to get order IDs from database: %v", err)
			continue
		}
		defer rows.Close()

		c.mutex.Lock()
		c.data = make(map[int]*model.Order) // Очищаем кэш перед обновлением

		for rows.Next() {
			var orderID int
			if err := rows.Scan(&orderID); err != nil {
				log.Printf("failed to scan order ID: %v", err)
				continue // Пропускаем ошибочный ID
			}

			// Получаем данные заказа из БД по ID
			order, err := c.db.GetOrderByID(orderID)
			if err != nil {
				log.Printf("failed to get order by ID %d: %v", orderID, err)
				continue // Пропускаем заказ, который не удалось получить
			}

			c.data[orderID] = order // Кэшируем заказ по order_id
		}
		c.mutex.Unlock()
	}
}

func (c *Cache) Close() error {
	c.ticker.Stop()
	return nil
}
