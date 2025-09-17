package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
	"github.com/gorilla/mux"
)

func (s *Server) createExchangeHandler(w http.ResponseWriter, r *http.Request) {
	var req ExchangeRequest
	if err := s.decodeJSON(r, &req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.validateExchangeRequest(&req); err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	exchangeID := generateExchangeID()

	symbol, side, err := s.determineSymbolAndSide(req.FromCurrency, req.ToCurrency)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	orderInfo := &orders.OrderInfo{
		ClOrdID:     generateOrderID(),
		Symbol:      symbol,
		Side:        side,
		OrderQty:    req.Amount,
		Price:       req.LimitPrice,
		OrdType:     s.getOrderType(req.Type),
		TimeInForce: "3",
	}

	if err := s.ordersClient.NewOrderSingle(orderInfo); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to create exchange order: %v", err), http.StatusInternalServerError)
		return
	}

	exchange := &ExchangeResponse{
		ExchangeID:   exchangeID,
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
		Amount:       req.Amount,
		Type:         req.Type,
		Status:       "pending",
		OrderID:      orderInfo.ClOrdID,
		Symbol:       symbol,
		Side:         side,
		CreatedAt:    time.Now(),
	}

	s.exchanges[exchangeID] = exchange

	log.Printf("[EXCHANGE] Created: %s (%s %s %.6f) -> Order: %s",
		exchangeID, req.FromCurrency, req.ToCurrency, req.Amount, orderInfo.ClOrdID)

	s.writeSuccess(w, exchange)
}

func (s *Server) getExchangeStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeID := vars["exchangeId"]

	exchange, exists := s.exchanges[exchangeID]
	if !exists {
		s.writeError(w, "Exchange operation not found", http.StatusNotFound)
		return
	}

	if order, exists := s.ordersClient.GetOrderStatus(exchange.OrderID); exists {
		exchange.Status = s.mapOrderStatusToExchangeStatus(order.Status)
	}

	s.writeSuccess(w, exchange)
}

func (s *Server) validateExchangeRequest(req *ExchangeRequest) error {
	if req.FromCurrency == "" {
		return fmt.Errorf("from_currency is required")
	}
	if req.ToCurrency == "" {
		return fmt.Errorf("to_currency is required")
	}
	if req.FromCurrency == req.ToCurrency {
		return fmt.Errorf("from_currency and to_currency must be different")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if req.Type != "market" && req.Type != "limit" {
		return fmt.Errorf("type must be 'market' or 'limit'")
	}
	if req.Type == "limit" && req.LimitPrice <= 0 {
		return fmt.Errorf("limit_price is required for limit orders")
	}
	return nil
}

func (s *Server) determineSymbolAndSide(fromCurrency, toCurrency string) (string, string, error) {
	availablePairs := map[string]bool{
		"BTC-EUR": true, "BTC-USD": true, "BTC-USDT": true, "BTC-GBP": true, "BTC-NZD": true,
		"ETH-USD": true, "ETH-GBP": true,
		"USDT-USD": true, "USDT-EUR": true, "USDT-GBP": true, "USDT-AUD": true, "USDT-PLN": true,
		"USDC-USD": true, "USDC-EUR": true, "USDC-GBP": true, "USDC-CHF": true,
		"LTC-GBP": true,
		"EUR-USD": true, "EUR-USDT": true, "EUR-CHF": true, "EUR-GBP": true, "EUR-NZD": true, "EUR-PLN": true,
		"GBP-USD": true, "GBP-CAD": true, "GBP-NZD": true, "GBP-PLN": true, "GBP-SGD": true,
		"USD-CAD": true, "USD-CHF": true, "USD-JPY": true, "USD-EUR": true,
		"NZD-USD": true,
	}

	directSymbol := fmt.Sprintf("%s-%s", fromCurrency, toCurrency)
	if availablePairs[directSymbol] {
		return directSymbol, "2", nil
	}

	reverseSymbol := fmt.Sprintf("%s-%s", toCurrency, fromCurrency)
	if availablePairs[reverseSymbol] {
		return reverseSymbol, "1", nil
	}

	return "", "", fmt.Errorf("currency pair not available: %s -> %s (try one of: %v)",
		fromCurrency, toCurrency, getAvailablePairsFor(fromCurrency, toCurrency, availablePairs))
}

func getAvailablePairsFor(from, to string, availablePairs map[string]bool) []string {
	var pairs []string
	for pair := range availablePairs {
		if strings.Contains(pair, from) || strings.Contains(pair, to) {
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

func (s *Server) getOrderType(exchangeType string) string {
	switch exchangeType {
	case "market":
		return "1"
	case "limit":
		return "2"
	default:
		return "1"
	}
}

func (s *Server) mapOrderStatusToExchangeStatus(orderStatus string) string {
	switch orderStatus {
	case "A":
		return "pending"
	case "0":
		return "pending"
	case "1":
		return "partial"
	case "2":
		return "completed"
	case "4":
		return "cancelled"
	case "8":
		return "failed"
	default:
		return "unknown"
	}
}

func generateExchangeID() string {
	return fmt.Sprintf("exch-%d", time.Now().UnixNano())
}
