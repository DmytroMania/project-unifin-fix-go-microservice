package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func (s *Server) subscribeMarketDataHandler(w http.ResponseWriter, r *http.Request) {
	var req MarketDataRequest
	if err := s.decodeJSON(r, &req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" {
		s.writeError(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	if err := s.mdClient.SubscribeToMarketDataWithWait(req.Symbol, 10*time.Second); err != nil {
		log.Printf("Subscribe to market data with timeout")
		s.writeError(w, fmt.Sprintf("Failed to subscribe: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeSuccess(w, fmt.Sprintf("Successfully subscribed to %s", req.Symbol))
}

func (s *Server) unsubscribeMarketDataHandler(w http.ResponseWriter, r *http.Request) {
	var req MarketDataRequest
	if err := s.decodeJSON(r, &req); err != nil {
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
