package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/marketdata"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
	"github.com/gorilla/mux"
)

type Server struct {
	mdClient     *marketdata.MarketDataClient
	ordersClient *orders.OrdersClient
	router       *mux.Router
	exchanges    map[string]*ExchangeResponse
}

func NewServer(mdClient *marketdata.MarketDataClient, ordersClient *orders.OrdersClient) *Server {
	server := &Server{
		mdClient:     mdClient,
		ordersClient: ordersClient,
		router:       mux.NewRouter(),
		exchanges:    make(map[string]*ExchangeResponse),
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")

	s.router.HandleFunc("/api/marketdata/subscribe", s.subscribeMarketDataHandler).Methods("POST")
	s.router.HandleFunc("/api/marketdata/unsubscribe", s.unsubscribeMarketDataHandler).Methods("POST")
	s.router.HandleFunc("/api/securities", s.requestSecuritiesHandler).Methods("GET")

	s.router.HandleFunc("/api/quotes", s.getQuotesHandler).Methods("GET")

	s.router.HandleFunc("/api/exchange", s.createExchangeHandler).Methods("POST")
	s.router.HandleFunc("/api/exchange/{exchangeId}", s.getExchangeStatusHandler).Methods("GET")

	s.router.HandleFunc("/api/orders", s.createOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/orders/{orderId}/cancel", s.cancelOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/orders/{orderId}/replace", s.replaceOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/orders/{orderId}", s.getOrderHandler).Methods("GET")
	s.router.HandleFunc("/api/orders/{orderId}/executions", s.getOrderExecutionsHandler).Methods("GET")
	s.router.HandleFunc("/api/orders", s.getAllOrdersHandler).Methods("GET")
	s.router.HandleFunc("/api/executions", s.getAllExecutionsHandler).Methods("GET")

	s.router.HandleFunc("/api/status", s.statusHandler).Methods("GET")
}

func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

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

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Data: "OK"})
}

func (s *Server) subscribeMarketDataHandler(w http.ResponseWriter, r *http.Request) {
	var req MarketDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		s.writeError(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	// Timeout 10 seconds to subscribe
	if err := s.mdClient.SubscribeToMarketDataWithWait(req.Symbol, 10*time.Second); err != nil {
		log.Printf("Subscribe to market data with timeout")

		s.writeError(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, fmt.Sprintf("Successfully subscribed to %s", req.Symbol))
}

func (s *Server) unsubscribeMarketDataHandler(w http.ResponseWriter, r *http.Request) {
	var req MarketDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		s.writeError(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	if err := s.mdClient.UnsubscribeFromMarketData(req.Symbol); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to unsubscribe: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, fmt.Sprintf("Unsubscribed from %s", req.Symbol))
}

func (s *Server) requestSecuritiesHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.mdClient.RequestSecurityList(); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to request securities: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, "Security list requested")
}

func (s *Server) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.validateOrderRequest(&req); err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	orderInfo := &orders.OrderInfo{
		ClOrdID:     generateOrderID(),
		Symbol:      req.Symbol,
		Side:        req.Side,
		OrderQty:    req.OrderQty,
		Price:       req.Price,
		OrdType:     req.OrdType,
		TimeInForce: req.TimeInForce,
	}

	if err := s.ordersClient.NewOrderSingle(orderInfo); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, map[string]string{"order_id": orderInfo.ClOrdID})
}

func (s *Server) cancelOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	order, exists := s.ordersClient.GetOrderStatus(orderID)
	if !exists {
		s.writeError(w, "Order not found", http.StatusNotFound)
		return
	}

	if err := s.ordersClient.CancelOrder(orderID, order.Symbol, order.Side); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to cancel order: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, "Order cancel request sent")
}

func (s *Server) replaceOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	origOrderID := vars["orderId"]

	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.validateOrderRequest(&req); err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	newOrderInfo := &orders.OrderInfo{
		ClOrdID:     generateOrderID(),
		Symbol:      req.Symbol,
		Side:        req.Side,
		OrderQty:    req.OrderQty,
		Price:       req.Price,
		OrdType:     req.OrdType,
		TimeInForce: req.TimeInForce,
	}

	if err := s.ordersClient.ReplaceOrder(origOrderID, newOrderInfo); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to replace order: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, map[string]string{"new_order_id": newOrderInfo.ClOrdID})
}

func (s *Server) getOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	order, exists := s.ordersClient.GetOrderStatus(orderID)
	if !exists {
		s.writeError(w, "Order not found", http.StatusNotFound)
		return
	}

	s.writeSuccess(w, order)
}

func (s *Server) getAllOrdersHandler(w http.ResponseWriter, r *http.Request) {
	orders := s.ordersClient.GetAllOrders()
	s.writeSuccess(w, orders)
}

func (s *Server) getOrderExecutionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	executions, exists := s.ordersClient.GetOrderExecutions(orderID)
	if !exists {
		s.writeError(w, "Order not found", http.StatusNotFound)
		return
	}

	s.writeSuccess(w, executions)
}

func (s *Server) getAllExecutionsHandler(w http.ResponseWriter, r *http.Request) {
	executions := s.ordersClient.GetAllExecutions()
	s.writeSuccess(w, executions)
}

func (s *Server) createExchangeHandler(w http.ResponseWriter, r *http.Request) {
	var req ExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := StatusResponse{
		MarketDataConnected: s.mdClient.IsConnected(),
		OrdersConnected:     s.ordersClient.IsConnected(),
		MarketDataLoggedIn:  s.mdClient.IsLoggedIn(),
		OrdersLoggedIn:      s.ordersClient.IsLoggedIn(),
		MarketDataSessionID: s.mdClient.GetSessionID().String(),
		OrdersSessionID:     s.ordersClient.GetSessionID().String(),
		MarketDataDetails:   s.mdClient.GetConnectionStatus(),
		OrdersDetails:       s.ordersClient.GetConnectionStatus(),
		Timestamp:           time.Now().UTC(),
	}

	s.writeSuccess(w, status)
}

func (s *Server) writeSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Data: data})
}

func (s *Server) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Success: false, Error: message})
}

func (s *Server) validateOrderRequest(req *OrderRequest) error {
	if req.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if req.Side != "1" && req.Side != "2" {
		return fmt.Errorf("side must be '1' (Buy) or '2' (Sell)")
	}
	if req.OrderQty <= 0 {
		return fmt.Errorf("order_qty must be positive")
	}
	if req.OrdType != "1" && req.OrdType != "2" && req.OrdType != "A" {
		return fmt.Errorf("ord_type must be '1' (Market), '2' (Limit), or 'A' (LimitAllIn)")
	}
	if req.OrdType == "2" && req.Price <= 0 {
		return fmt.Errorf("price is required for limit orders")
	}
	if req.TimeInForce == "" {
		req.TimeInForce = "1"
	}

	return nil
}

func (s *Server) getQuotesHandler(w http.ResponseWriter, r *http.Request) {
	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		s.writeError(w, "symbols parameter is required", http.StatusBadRequest)
		return
	}

	var symbols []string
	for _, symbol := range strings.Split(symbolsParam, ",") {
		symbol = strings.TrimSpace(symbol)
		if symbol != "" {
			symbols = append(symbols, symbol)
		}
	}

	if len(symbols) == 0 {
		s.writeError(w, "at least one symbol is required", http.StatusBadRequest)
		return
	}

	quotes := s.mdClient.GetQuotesWithWait(symbols, 10*time.Second)

	response := make(map[string]interface{})

	for _, symbol := range symbols {
		if quote, exists := quotes[symbol]; exists && quote != nil {
			response[symbol] = quote
		} else {
			response[symbol] = nil
		}
	}

	s.writeSuccess(w, response)
}

func generateOrderID() string {
	return fmt.Sprintf("ord-%d", time.Now().UnixNano())
}

func generateExchangeID() string {
	return fmt.Sprintf("exch-%d", time.Now().UnixNano())
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
	/* from BCB securities list */
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
		/* fromCurrency sell to toCurrency */
		return directSymbol, "2", nil
	}

	reverseSymbol := fmt.Sprintf("%s-%s", toCurrency, fromCurrency)
	if availablePairs[reverseSymbol] {
		/* toCurrency buy from fromCurrency */
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
