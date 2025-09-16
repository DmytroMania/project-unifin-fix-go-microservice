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
	log.Println("[EVENT (MicroserviceStarting)]: BCB Markets FIX Microservice")

	mdClient := marketdata.NewMarketDataClient()
	ordersClient := orders.NewOrdersClient()

	log.Println("[EVENT (MarketDataClientStarting)]")

	if err := mdClient.Start("config/market_data.cfg"); err != nil {
		log.Fatalf("Failed to start Market Data client: %v", err)
	}

	defer mdClient.Stop()

	time.Sleep(3 * time.Second)

	log.Println("[EVENT (OrdersClientStarting)]")

	if err := ordersClient.Start("config/order_entry.cfg"); err != nil {
		log.Fatalf("Failed to start Orders client: %v", err)
	}

	defer ordersClient.Stop()

	time.Sleep(3 * time.Second)

	apiServer := api.NewServer(mdClient, ordersClient)

	go func() {
		log.Println("[EVENT (HTTPServerStarting)]: Port 8080")

		if err := apiServer.Start(8080); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	log.Println("[EVENT (MicroserviceRunning)]: BCB Markets FIX Microservice")

	<-c
	log.Println("[EVENT (MicroserviceShuttingDown)]")
}
