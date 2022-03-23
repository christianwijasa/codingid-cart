package main

import (
	"database/sql"

	"github.com/google/uuid"
)

type cart struct {
	ID    string     `json:"id"`
	Total int        `json:"total"`
	Items []cartItem `json:"items"`
}

type cartItem struct {
	ID          string `json:"id"`
	CartID      string `json:"cart_id"`
	SKU         string `json:"sku"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
}

func getCarts(db *sql.DB, limit int, offset int) ([]cart, error) {
	rows, err := db.Query("SELECT id, total FROM carts LIMIT ? OFFSET ?",
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	carts := []cart{}

	for rows.Next() {
		var c cart
		if err := rows.Scan(&c.ID, &c.Total); err != nil {
			return nil, err
		}

		if err = c.getCartByID(db); err != nil {
			return nil, err
		}

		carts = append(carts, c)
	}

	return carts, nil
}

func (c *cart) getCartByID(db *sql.DB) error {
	err := db.QueryRow("SELECT id, total FROM carts WHERE id=?",
		c.ID).Scan(&c.ID, &c.Total)
	if err != nil {
		return err
	}

	cartItems, err := getCartItemsByCartID(db, c.ID)
	if err != nil {
		return err
	}

	c.Items = cartItems
	return nil
}

func getCartItemsByCartID(db *sql.DB, cartID string) ([]cartItem, error) {
	rows, err := db.Query(
		"SELECT id, cart_id, sku, product_name, quantity FROM cart_items WHERE cart_id=?",
		cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cartItems := []cartItem{}

	for rows.Next() {
		var ci cartItem
		if err := rows.Scan(&ci.ID, &ci.CartID, &ci.SKU, &ci.ProductName, &ci.Quantity); err != nil {
			return nil, err
		}
		cartItems = append(cartItems, ci)
	}

	return cartItems, nil
}

func (c *cart) createCart(db *sql.DB) error {
	c.ID = uuid.New().String()

	_, err := db.Exec("INSERT INTO carts(id) VALUES(?)", c.ID)
	if err != nil {
		return err
	}

	return db.QueryRow("SELECT id, total FROM carts WHERE id=?",
		c.ID).Scan(&c.ID, &c.Total)
}

func (ci *cartItem) addItemToCart(db *sql.DB) (cart, error) {
	_, err := db.Query("SELECT id FROM carts WHERE id=?",
		ci.CartID)
	if err != nil {
		return cart{}, err
	}

	ci.ID = uuid.New().String()

	_, err = db.Exec(`
		INSERT INTO cart_items(id, cart_id, sku, product_name, quantity)
		VALUES(?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE quantity=quantity+?`,
		ci.ID, ci.CartID, ci.SKU, ci.ProductName, ci.Quantity, ci.Quantity)
	if err != nil {
		return cart{}, err
	}

	_, err = db.Exec("UPDATE carts SET total=total+? WHERE id=?",
		ci.Quantity, ci.CartID)
	if err != nil {
		return cart{}, err
	}

	c := cart{ID: ci.CartID}

	if err = c.getCartByID(db); err != nil {
		return cart{}, err
	}

	return c, nil
}

func (c *cart) deleteCart(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM cart_items WHERE cart_id=?", c.ID)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM carts WHERE id=?", c.ID)
	if err != nil {
		return err
	}

	return nil
}

func (ci *cartItem) deleteCartItem(db *sql.DB) error {
	err := db.QueryRow("SELECT id, cart_id, quantity FROM cart_items WHERE id=? AND cart_id=?",
		ci.ID, ci.CartID).Scan(&ci.ID, &ci.CartID, &ci.Quantity)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE carts SET total=total-? WHERE id=?", ci.Quantity, ci.CartID)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM cart_items WHERE id=? AND cart_id=?", ci.ID, ci.CartID)
	if err != nil {
		return err
	}

	return nil
}
