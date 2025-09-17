package api

import "time"

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type MarketDataRequest struct {
	Symbol string `json:"symbol"`
}

type OrderRequest struct {
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`
	OrderQty    float64 `json:"order_qty"`
	Price       float64 `json:"price,omitempty"`
	OrdType     string  `json:"ord_type"`
	TimeInForce string  `json:"time_in_force"`
}

type ExchangeRequest struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Amount       float64 `json:"amount"`
	Type         string  `json:"type"`
	LimitPrice   float64 `json:"limit_price,omitempty"`
}

type ExchangeResponse struct {
	ExchangeID   string    `json:"exchange_id"`
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Amount       float64   `json:"amount"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	OrderID      string    `json:"order_id"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	CreatedAt    time.Time `json:"created_at"`
}

type StatusResponse struct {
	MarketDataConnected bool                   `json:"market_data_connected"`
	OrdersConnected     bool                   `json:"orders_connected"`
	MarketDataLoggedIn  bool                   `json:"market_data_logged_in"`
	OrdersLoggedIn      bool                   `json:"orders_logged_in"`
	MarketDataSessionID string                 `json:"market_data_session_id"`
	OrdersSessionID     string                 `json:"orders_session_id"`
	MarketDataDetails   map[string]interface{} `json:"market_data_details"`
	OrdersDetails       map[string]interface{} `json:"orders_details"`
	Timestamp           time.Time              `json:"timestamp"`
}
