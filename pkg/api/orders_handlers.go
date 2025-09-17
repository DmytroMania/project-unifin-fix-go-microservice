package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
	"github.com/gorilla/mux"
)

func (s *Server) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := s.decodeJSON(r, &req); err != nil {
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
	if err := s.decodeJSON(r, &req); err != nil {
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
