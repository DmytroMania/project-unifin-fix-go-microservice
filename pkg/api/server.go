package api

import (
	"fmt"
	"log"
	"net/http"

	"bcb-fix-microservice/pkg/marketdata"
	"bcb-fix-microservice/pkg/orders"
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
