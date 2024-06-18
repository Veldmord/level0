package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"Level0/model"

	_ "github.com/lib/pq"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dsn string) (*Database, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) SaveOrder(order *model.Order) error {
	tx, err := d.db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Гарантированный откат в случае ошибки

	log.Printf("Inserting order: %+v", order)

	var orderID int
	err = tx.QueryRowContext(context.Background(),
		`INSERT INTO "Order" (order_uid, track_number, entry, locale, internal_sign, customer_id, 
                     delivery_service, shard_key, sm_id, date_created, oof_shard) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING order_id`,
		order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSign,
		order.CustomerID, order.DeliveryService, order.ShardKey, order.SMID, order.DateCreated, order.OOFShard).Scan(&orderID)
	if err != nil {
		log.Printf("Error inserting order: %v", err)
		return fmt.Errorf("failed to insert order: %w", err)
	}

	log.Printf("Inserted order with ID: %d", orderID)

	_, err = tx.ExecContext(context.Background(),
		`INSERT INTO "Delivery" (order_id, name, phone, zip, city, address, region, email) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		orderID, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City,
		order.Delivery.Address, order.Delivery.Region, order.Delivery.Email)
	if err != nil {
		log.Printf("Error inserting delivery: %v", err)
		return fmt.Errorf("failed to insert delivery: %w", err)
	}

	_, err = tx.ExecContext(context.Background(),
		`INSERT INTO "Payment" (order_id, transaction, request_id, currency, provider, amount, 
                     payment_dt, bank, delivery_cost, goods_total, custom_fee) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		orderID, order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency, order.Payment.Provider,
		order.Payment.Amount, order.Payment.PaymentDT, order.Payment.Bank, order.Payment.DeliveryCost,
		order.Payment.GoodsTotal, order.Payment.CustomFee)
	if err != nil {
		log.Printf("Error inserting payment: %v", err)
		return fmt.Errorf("failed to insert payment: %w", err)
	}

	for _, item := range order.Items {
		_, err = tx.ExecContext(context.Background(),
			`INSERT INTO "Item" (order_id, chrt_id, track_number, price, rid, name, sale, size, 
                         total_price, nm_id, brand, status) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			orderID, item.ChrtID, item.TrackNumber, item.Price, item.RID, item.Name, item.Sale,
			item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status)
		if err != nil {
			log.Printf("Error inserting item: %v", err)
			return fmt.Errorf("failed to insert item: %w", err)
		}
	}

	log.Println("Committing transaction")
	return tx.Commit()
}

func (d *Database) GetOrderByID(orderID int) (*model.Order, error) {
	// Получение данных из таблицы "Order"
	var order model.Order
	err := d.db.QueryRowContext(context.Background(),
		`SELECT order_uid, track_number, entry, locale, internal_sign, customer_id, 
			 delivery_service, shard_key, sm_id, date_created, oof_shard 
			 FROM "Order" WHERE order_id = $1`,
		orderID).Scan(
		&order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale, &order.InternalSign,
		&order.CustomerID, &order.DeliveryService, &order.ShardKey, &order.SMID, &order.DateCreated, &order.OOFShard)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Получение данных из таблицы "Delivery"
	err = d.db.QueryRowContext(context.Background(),
		`SELECT name, phone, zip, city, address, region, email 
			 FROM "Delivery" WHERE order_id = $1`,
		orderID).Scan(
		&order.Delivery.Name, &order.Delivery.Phone, &order.Delivery.Zip, &order.Delivery.City,
		&order.Delivery.Address, &order.Delivery.Region, &order.Delivery.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get delivery: %w", err)
	}

	// Получение данных из таблицы "Payment"
	err = d.db.QueryRowContext(context.Background(),
		`SELECT transaction, request_id, currency, provider, amount, payment_dt, 
			 bank, delivery_cost, goods_total, custom_fee 
			 FROM "Payment" WHERE order_id = $1`,
		orderID).Scan(
		&order.Payment.Transaction, &order.Payment.RequestID, &order.Payment.Currency,
		&order.Payment.Provider, &order.Payment.Amount, &order.Payment.PaymentDT,
		&order.Payment.Bank, &order.Payment.DeliveryCost, &order.Payment.GoodsTotal,
		&order.Payment.CustomFee)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	// Получение данных из таблицы "Item"
	rows, err := d.db.QueryContext(context.Background(),
		`SELECT chrt_id, track_number, price, rid, name, sale, size, 
			 total_price, nm_id, brand, status 
			 FROM "Item" WHERE order_id = $1`,
		orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item model.Item
		err = rows.Scan(
			&item.ChrtID, &item.TrackNumber, &item.Price, &item.RID, &item.Name,
			&item.Sale, &item.Size, &item.TotalPrice, &item.NmID, &item.Brand, &item.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		order.Items = append(order.Items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate over items: %w", err)
	}

	return &order, nil
}

func (d *Database) GetLastOrderID() (int, error) {
	var orderID int
	err := d.db.QueryRow("SELECT lastval()").Scan(&orderID)
	if err != nil {
		return 0, fmt.Errorf("failed to get last order ID: %w", err)
	}
	return orderID, nil
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}

func (d *Database) Close() error {
	return d.db.Close()
}
