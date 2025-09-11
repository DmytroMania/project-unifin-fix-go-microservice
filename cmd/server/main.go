package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/api"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/marketdata"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
)

func main() {
	log.Println("Starting BCB Markets FIX Microservice...")

	mdClient := marketdata.NewMarketDataClient()
	ordersClient := orders.NewOrdersClient()

	log.Println("Starting Market Data client...")
	if err := mdClient.Start("config/market_data.cfg"); err != nil {
		log.Fatalf("Failed to start Market Data client: %v", err)
	}
	defer mdClient.Stop()

	time.Sleep(3 * time.Second)

	log.Println("Starting Orders client...")
	if err := ordersClient.Start("config/order_entry.cfg"); err != nil {
		log.Fatalf("Failed to start Orders client: %v", err)
	}
	defer ordersClient.Stop()

	time.Sleep(3 * time.Second)

	apiServer := api.NewServer(mdClient, ordersClient)

	go func() {
		log.Println("Starting HTTP API server on port 8080...")
		if err := apiServer.Start(8080); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("BCB Markets FIX Microservice is running...")

	<-c
	log.Println("\nShutting down...")
}
