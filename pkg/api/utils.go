package api

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) writeSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Data: data})
}

func (s *Server) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Success: false, Error: message})
}

func (s *Server) decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Data: "OK"})
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
