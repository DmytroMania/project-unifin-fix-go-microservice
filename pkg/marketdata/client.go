package marketdata

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/bcb"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/logging"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/tag"
)

type MarketDataClient struct {
	*bcb.BCBApplication
	initiator     *quickfix.Initiator
	subscriptions map[string]bool
}

func NewMarketDataClient() *MarketDataClient {
	return &MarketDataClient{
		BCBApplication: bcb.NewBCBApplication(),
		subscriptions:  make(map[string]bool),
	}
}

func (client *MarketDataClient) Start(configFile string) error {
	file, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	cfg, err := quickfix.ParseSettings(file)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	storeFactory := quickfix.NewMemoryStoreFactory()
	logFactory := logging.NewDebugLogFactory("log")

	client.initiator, err = quickfix.NewInitiator(client, storeFactory, cfg, logFactory)
	if err != nil {
		return fmt.Errorf("failed to create initiator: %w", err)
	}

	if err := client.initiator.Start(); err != nil {
		return fmt.Errorf("failed to start initiator: %w", err)
	}

	log.Println("Market Data client started successfully")
	return nil
}

func (client *MarketDataClient) Stop() {
	if client.initiator != nil {
		client.initiator.Stop()
		log.Println("Market Data client stopped")
	}
}

func (client *MarketDataClient) OnLogon(sessionID quickfix.SessionID) {
	client.BCBApplication.OnLogon(sessionID)

	go func() {
		time.Sleep(2 * time.Second)
		if err := client.RequestSecurityList(); err != nil {
			log.Printf("Failed to request security list: %v", err)
		}
	}()
}

func (client *MarketDataClient) RequestSecurityList() error {
	message := quickfix.NewMessage()
	message.Body.SetString(tag.MsgType, "x")
	message.Body.SetString(tag.SecurityReqID, generateRequestID())
	message.Body.SetInt(tag.SecurityListRequestType, 4)

	return quickfix.SendToTarget(message, client.GetSessionID())
}

func (client *MarketDataClient) SubscribeToMarketData(symbol string) error {
	if client.subscriptions[symbol] {
		return fmt.Errorf("already subscribed to %s", symbol)
	}

	message := quickfix.NewMessage()

	message.Body.SetString(tag.MsgType, "V")
	message.Body.SetString(tag.MDReqID, generateRequestID())
	message.Body.SetInt(tag.SubscriptionRequestType, 1)
	message.Body.SetInt(tag.MarketDepth, 0)
	message.Body.SetInt(tag.MDUpdateType, 0)
	message.Body.SetInt(tag.MDQuoteType, 1)

	message.Body.SetString(tag.Symbol, symbol)

	message.Body.SetInt(tag.NoMDEntryTypes, 2)

	message.Body.SetString(tag.MDEntryType, "0")
	message.Body.SetString(tag.MDEntryType, "1")

	message.Header.SetString(tag.MsgType, "V")

	log.Printf("Sending Market Data Request for %s", symbol)
	log.Printf("Message: %s", message.String())

	sessionID := client.GetSessionID()
	if sessionID.SenderCompID == "" || sessionID.TargetCompID == "" {
		return fmt.Errorf("no active session")
	}

	if err := quickfix.SendToTarget(message, sessionID); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", symbol, err)
	}

	client.subscriptions[symbol] = true
	log.Printf("Subscribed to market data for %s", symbol)
	return nil
}

func (client *MarketDataClient) UnsubscribeFromMarketData(symbol string) error {
	if !client.subscriptions[symbol] {
		return fmt.Errorf("not subscribed to %s", symbol)
	}

	message := quickfix.NewMessage()
	message.Body.SetString(tag.MsgType, "V") // Market Data Request
	message.Body.SetString(tag.MDReqID, generateRequestID())
	message.Body.SetInt(tag.SubscriptionRequestType, 2) // Unsubscribe
	message.Body.SetString(tag.Symbol, symbol)

	if err := quickfix.SendToTarget(message, client.GetSessionID()); err != nil {
		return fmt.Errorf("failed to unsubscribe from %s: %w", symbol, err)
	}

	delete(client.subscriptions, symbol)
	log.Printf("Unsubscribed from market data for %s", symbol)
	return nil
}

func (client *MarketDataClient) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)

	switch msgType {
	case "W":
		client.handleMarketDataSnapshot(message)
	case "Y":
		client.handleSecurityListResponse(message)
	case "y":
		client.handleMarketDataReject(message)
	default:
		return client.BCBApplication.FromApp(message, sessionID)
	}

	return nil
}

func (client *MarketDataClient) handleMarketDataSnapshot(message *quickfix.Message) {
	symbol, _ := message.Body.GetString(tag.Symbol)
	mdReqID, _ := message.Body.GetString(tag.MDReqID)
	noMDEntries, _ := message.Body.GetInt(tag.NoMDEntries)

	log.Printf("Market Data Snapshot for %s (ReqID: %s, Entries: %d)", symbol, mdReqID, noMDEntries)

	// TODO
}

func (client *MarketDataClient) handleSecurityListResponse(message *quickfix.Message) {
	secReqID, _ := message.Body.GetString(tag.SecurityReqID)
	secResponseID, _ := message.Body.GetString(tag.SecurityResponseID)
	result, _ := message.Body.GetInt(tag.SecurityRequestResult)

	log.Printf("Security List Response (ReqID: %s, ResponseID: %s, Result: %d)", secReqID, secResponseID, result)

	if result == 0 {
		noRelatedSym, _ := message.Body.GetInt(tag.NoRelatedSym)
		log.Printf("Received %d instruments", noRelatedSym)

		// TODO
	}
}

func (client *MarketDataClient) handleMarketDataReject(message *quickfix.Message) {
	mdReqID, _ := message.Body.GetString(tag.MDReqID)
	text, _ := message.Body.GetString(tag.Text)

	log.Printf("Market Data Request Rejected (ReqID: %s): %s", mdReqID, text)
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}
