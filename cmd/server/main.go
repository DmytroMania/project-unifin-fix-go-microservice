package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/api"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/marketdata"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/orders"
)

func main() {
	log.Println("[EVENT (MicroserviceStarting)]: BCB Markets FIX Microservice")

	port := getEnvInt("PORT", 8085)
	mdConfigPath := getEnvString("MD_CONFIG_PATH", "config/market_data.cfg")
	oeConfigPath := getEnvString("OE_CONFIG_PATH", "config/order_entry.cfg")

	mdClient := marketdata.NewMarketDataClient()
	ordersClient := orders.NewOrdersClient()

	log.Println("[EVENT (MarketDataClientStarting)]")

	if err := mdClient.Start(mdConfigPath); err != nil {
		log.Fatalf("Failed to start Market Data client: %v", err)
	}

	defer mdClient.Stop()

	time.Sleep(3 * time.Second)

	log.Println("[EVENT (OrdersClientStarting)]")

	if err := ordersClient.Start(oeConfigPath); err != nil {
		log.Fatalf("Failed to start Orders client: %v", err)
	}

	defer ordersClient.Stop()

	time.Sleep(3 * time.Second)

	apiServer := api.NewServer(mdClient, ordersClient)

	go func() {
		log.Printf("[EVENT (HTTPServerStarting)]: Port %d", port)

		if err := apiServer.Start(port); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("[EVENT (MicroserviceRunning)]: BCB Markets FIX Microservice")

	<-c
	log.Println("[EVENT (MicroserviceShuttingDown)]")
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
