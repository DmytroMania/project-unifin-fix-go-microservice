package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/marketdata"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
	"github.com/gorilla/mux"
)

type Server struct {
	mdClient     *marketdata.MarketDataClient
	ordersClient *orders.OrdersClient
	router       *mux.Router
}

func NewServer(mdClient *marketdata.MarketDataClient, ordersClient *orders.OrdersClient) *Server {
	server := &Server{
		mdClient:     mdClient,
		ordersClient: ordersClient,
		router:       mux.NewRouter(),
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")

	s.router.HandleFunc("/api/v1/marketdata/subscribe", s.subscribeMarketDataHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/marketdata/unsubscribe", s.unsubscribeMarketDataHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/securities", s.requestSecuritiesHandler).Methods("GET")

	s.router.HandleFunc("/api/v1/orders", s.createOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/orders/{orderId}/cancel", s.cancelOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/orders/{orderId}/replace", s.replaceOrderHandler).Methods("POST")
	s.router.HandleFunc("/api/v1/orders/{orderId}", s.getOrderHandler).Methods("GET")
	s.router.HandleFunc("/api/v1/orders", s.getAllOrdersHandler).Methods("GET")

	s.router.HandleFunc("/api/v1/status", s.statusHandler).Methods("GET")
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

	if err := s.mdClient.SubscribeToMarketData(req.Symbol); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, fmt.Sprintf("Subscribed to %s", req.Symbol))
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

func generateOrderID() string {
	return fmt.Sprintf("ord-%d", time.Now().UnixNano())
}
