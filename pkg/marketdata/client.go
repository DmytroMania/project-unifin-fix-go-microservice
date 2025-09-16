package marketdata

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/bcb"
	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/logging"
	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix44/marketdatarequest"
	"github.com/quickfixgo/fix44/marketdatasnapshotfullrefresh"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/tag"
)

type MarketDataClient struct {
	*bcb.BCBApplication
	initiator     *quickfix.Initiator
	subscriptions map[string]string
	quotes        map[string]Quote
	subscribers   map[string]int
	quoteChannels map[string]chan struct{}
	subChannels   map[string]chan error
}

type Quote struct {
	Symbol    string    `json:"symbol"`
	Bid       float64   `json:"bid"`
	Ask       float64   `json:"ask"`
	Last      float64   `json:"last"`
	Size      float64   `json:"size"`
	Timestamp time.Time `json:"timestamp"`
	Stale     bool      `json:"stale"`
}

func NewMarketDataClient() *MarketDataClient {
	return &MarketDataClient{
		BCBApplication: bcb.NewBCBApplication(),
		subscriptions:  make(map[string]string),
		quotes:         make(map[string]Quote),
		subscribers:    make(map[string]int),
		quoteChannels:  make(map[string]chan struct{}),
		subChannels:    make(map[string]chan error),
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

	client.BCBApplication.SetInitiator(client.initiator)

	if err := client.initiator.Start(); err != nil {
		return fmt.Errorf("failed to start initiator: %w", err)
	}

	log.Println("[EVENT (MarketDataClientStarted)]")
	return nil
}

func (client *MarketDataClient) Stop() {
	if client.initiator != nil {
		client.initiator.Stop()
		log.Println("[EVENT (MarketDataClientStopped)]")
	}
}

func (client *MarketDataClient) OnLogon(sessionID quickfix.SessionID) {
	client.BCBApplication.OnLogon(sessionID)

	go func() {
		time.Sleep(2 * time.Second)
		if err := client.RequestSecurityList(); err != nil {
			log.Printf("[ERROR (SecurityListRequestFailed)]: %v", err)
		}
	}()
}

func (client *MarketDataClient) RequestSecurityList() error {
	message := quickfix.NewMessage()
	message.Header.SetString(tag.MsgType, "x")

	message.Body.SetString(tag.SecurityReqID, generateRequestID())
	message.Body.SetInt(tag.SecurityListRequestType, 4)

	return quickfix.SendToTarget(message, client.GetSessionID())
}

func (client *MarketDataClient) SubscribeToMarketData(symbol string) error {
	if _, exists := client.subscriptions[symbol]; exists {
		return fmt.Errorf("already subscribed to %s", symbol)
	}

	mdReqID := generateRequestID()

	mdReq := marketdatarequest.New(
		field.NewMDReqID(mdReqID),
		field.NewSubscriptionRequestType("1"),
		field.NewMarketDepth(1),
	)

	mdReq.SetMDUpdateType(enum.MDUpdateType_FULL_REFRESH)

	noRelatedSym := marketdatarequest.NewNoRelatedSymRepeatingGroup()
	rel := noRelatedSym.Add()
	rel.SetSymbol(symbol)
	mdReq.SetNoRelatedSym(noRelatedSym)

	noMDEntryTypes := marketdatarequest.NewNoMDEntryTypesRepeatingGroup()
	entryBid := noMDEntryTypes.Add()
	entryBid.SetMDEntryType("0")

	entryOffer := noMDEntryTypes.Add()
	entryOffer.SetMDEntryType("1")

	mdReq.SetNoMDEntryTypes(noMDEntryTypes)

	msg := mdReq.ToMessage()

	msg.Body.SetInt(1070, 1)

	log.Printf("[SEND (MarketDataRequest)]: symbol=%s", symbol)

	sessionID := client.GetSessionID()
	if sessionID.SenderCompID == "" || sessionID.TargetCompID == "" {
		return fmt.Errorf("no active session")
	}

	if err := quickfix.SendToTarget(msg, sessionID); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", symbol, err)
	}

	client.subChannels[symbol] = make(chan error, 1)
	client.subscriptions[symbol] = mdReqID
	log.Printf("[EVENT (MarketDataRequestSent)]: %s", symbol)
	return nil
}

func (client *MarketDataClient) SubscribeToMarketDataWithWait(symbol string, timeout time.Duration) error {
	if err := client.SubscribeToMarketData(symbol); err != nil {
		return err
	}

	select {
	case err := <-client.subChannels[symbol]:
		if err != nil {
			delete(client.subscriptions, symbol)
			delete(client.subChannels, symbol)
			return err
		}
		log.Printf("[EVENT (MarketDataSubscribed)]: %s", symbol)
		return nil
	case <-time.After(timeout):
		delete(client.subscriptions, symbol)
		delete(client.subChannels, symbol)
		return fmt.Errorf("timeout waiting for subscription confirmation for %s", symbol)
	}
}

func (client *MarketDataClient) UnsubscribeFromMarketData(symbol string) error {
	mdReqID, ok := client.subscriptions[symbol]

	if !ok {
		return fmt.Errorf("not subscribed to %s", symbol)
	}

	mdReq := marketdatarequest.New(
		field.NewMDReqID(mdReqID),
		field.NewSubscriptionRequestType("2"),
		field.NewMarketDepth(1),
	)

	mdReq.SetMDUpdateType(enum.MDUpdateType_FULL_REFRESH)

	noRelatedSym := marketdatarequest.NewNoRelatedSymRepeatingGroup()
	rel := noRelatedSym.Add()
	rel.SetSymbol(symbol)
	mdReq.SetNoRelatedSym(noRelatedSym)

	noMDEntryTypes := marketdatarequest.NewNoMDEntryTypesRepeatingGroup()
	e1 := noMDEntryTypes.Add()
	e1.SetMDEntryType("0")
	e2 := noMDEntryTypes.Add()
	e2.SetMDEntryType("1")
	mdReq.SetNoMDEntryTypes(noMDEntryTypes)

	msg := mdReq.ToMessage()
	msg.Body.SetInt(1070, 1)

	log.Printf("[SEND (MarketDataRequest UNSUBSCRIBE)]: symbol=%s mdReqID=%s", symbol, mdReqID)

	sessionID := client.GetSessionID()
	if sessionID.SenderCompID == "" || sessionID.TargetCompID == "" {
		return fmt.Errorf("no active session")
	}

	if err := quickfix.SendToTarget(msg, sessionID); err != nil {
		return fmt.Errorf("failed to unsubscribe from %s: %w", symbol, err)
	}

	delete(client.subscriptions, symbol)

	log.Printf("[EVENT (MarketDataUnsubscribed)]: %s (mdReqID=%s)", symbol, mdReqID)
	return nil
}

func (client *MarketDataClient) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, err := message.Body.GetString(tag.MsgType)
	if err != nil || msgType == "" {
		msgType, err = message.Header.GetString(tag.MsgType)
		if err != nil || msgType == "" {
			msgType, _ = message.Body.GetString(35)
		}
	}

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
	snapshot := marketdatasnapshotfullrefresh.FromMessage(message)

	symbol, _ := snapshot.GetSymbol()
	mdReqID, _ := snapshot.GetMDReqID()
	noMDEntries, _ := snapshot.GetNoMDEntries()

	log.Printf("[RECEIVE (MarketDataSnapshot)]: %s (ReqID: %s, Entries: %d)", symbol, mdReqID, noMDEntries)

	if noMDEntries.Len() == 0 {
		log.Printf("[WARNING] No market data available for symbol: %s (ReqID: %s)", symbol, mdReqID)

		if reqID, exists := client.subscriptions[symbol]; exists && reqID == mdReqID {
			delete(client.subscriptions, symbol)
			log.Printf("[EVENT (MarketDataSubscriptionRemoved)]: %s - Symbol not found", symbol)

			if ch, exists := client.subChannels[symbol]; exists {
				select {
				case ch <- fmt.Errorf("symbol %s not found", symbol):
				default:
				}
				delete(client.subChannels, symbol)
			}
		}

		if ch, exists := client.quoteChannels[symbol]; exists {
			select {
			case ch <- struct{}{}:
			default:
			}
		}

		return
	}

	if ch, exists := client.subChannels[symbol]; exists {
		select {
		case ch <- nil:
		default:
		}
		delete(client.subChannels, symbol)
	}

	client.parseAndStoreQuotes(snapshot, symbol)
}

func (client *MarketDataClient) parseAndStoreQuotes(snapshot marketdatasnapshotfullrefresh.MarketDataSnapshotFullRefresh, symbol string) {
	var bid, ask, last, size float64

	group, _ := snapshot.GetNoMDEntries()
	for i := 0; i < group.Len(); i++ {
		entry := group.Get(i)

		mdEntryType, _ := entry.GetMDEntryType()
		mdEntryPx, _ := entry.GetMDEntryPx()
		mdEntrySize, _ := entry.GetMDEntrySize()

		mdEntryPxFloat, _ := mdEntryPx.Float64()
		mdEntrySizeFloat, _ := mdEntrySize.Float64()

		switch mdEntryType {
		case "0":
			bid = mdEntryPxFloat
			size = mdEntrySizeFloat
		case "1":
			ask = mdEntryPxFloat
		case "2":
			last = mdEntryPxFloat
		default:
		}
	}

	if bid > 0 || ask > 0 || last > 0 {
		quote := Quote{
			Symbol:    symbol,
			Bid:       bid,
			Ask:       ask,
			Last:      last,
			Size:      size,
			Timestamp: time.Now(),
			Stale:     false,
		}

		hadQuote := client.quotes[symbol].Symbol != ""

		client.quotes[symbol] = quote

		if !hadQuote {
			if ch, exists := client.quoteChannels[symbol]; exists {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}

		log.Printf("[STORE (Quote)]: %s - Bid: %.6f, Ask: %.6f, Last: %.6f, Size: %.6f",
			symbol, bid, ask, last, size)
	}
}

func (client *MarketDataClient) handleSecurityListResponse(message *quickfix.Message) {
	secReqID, _ := message.Body.GetString(tag.SecurityReqID)
	secResponseID, _ := message.Body.GetString(tag.SecurityResponseID)
	result, _ := message.Body.GetInt(tag.SecurityRequestResult)

	log.Printf("[RECEIVE (SecurityListResponse)]: ReqID=%s, ResponseID=%s, Result=%d", secReqID, secResponseID, result)

	if result == 0 {
		noRelatedSym, _ := message.Body.GetInt(tag.NoRelatedSym)
		log.Printf("[RECEIVE (InstrumentsCount)]: %d", noRelatedSym)

		// TODO
	}
}

func (client *MarketDataClient) handleMarketDataReject(message *quickfix.Message) {
	mdReqID, _ := message.Body.GetString(tag.MDReqID)
	text, _ := message.Body.GetString(tag.Text)

	log.Printf("[RECEIVE (MarketDataRequestRejected)]: ReqID=%s, Reason=%s", mdReqID, text)

	for symbol, reqID := range client.subscriptions {
		if reqID == mdReqID {
			delete(client.subscriptions, symbol)
			log.Printf("[EVENT (MarketDataSubscriptionRemoved)]: %s - Request rejected", symbol)

			if ch, exists := client.subChannels[symbol]; exists {
				select {
				case ch <- fmt.Errorf("subscription rejected: %s", text):
				default:
				}
				delete(client.subChannels, symbol)
			}

			if ch, exists := client.quoteChannels[symbol]; exists {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
			break
		}
	}
}

func (client *MarketDataClient) GetConnectionStatus() map[string]interface{} {
	return client.BCBApplication.GetConnectionStatus()
}

/*
func (client *MarketDataClient) GetQuotes(symbols []string) map[string]*Quote {
	result := make(map[string]*Quote)

	log.Printf("[DEBUG] GetQuotes called for symbols: %v", symbols)

	for _, symbol := range symbols {
		client.subscribers[symbol]++

		if _, exists := client.subscriptions[symbol]; !exists {
			log.Printf("[DEBUG] No subscription for %s, creating one", symbol)
			go client.SubscribeToMarketData(symbol)
		} else {
			log.Printf("[DEBUG] Subscription exists for %s", symbol)
		}

		if quote, exists := client.quotes[symbol]; exists {
			if time.Since(quote.Timestamp) > 90*time.Second {
				quote.Stale = true
				log.Printf("[DEBUG] Quote for %s is stale", symbol)
			}

			log.Printf("[DEBUG] Found quote for %s: Bid=%.6f, Ask=%.6f, Last=%.6f", symbol, quote.Bid, quote.Ask, quote.Last)
			result[symbol] = &quote
		} else {
			log.Printf("[DEBUG] No quote found for %s, waiting for first data...", symbol)
			result[symbol] = nil
		}
	}

	log.Printf("[DEBUG] Returning quotes: %+v", result)
	return result
}
*/

func (client *MarketDataClient) GetQuotesWithWait(symbols []string, timeout time.Duration) map[string]*Quote {
	result := make(map[string]*Quote)
	waitingSymbols := make([]string, 0)

	log.Printf("[DEBUG] GetQuotesWithWait called for symbols: %v", symbols)

	for _, symbol := range symbols {
		client.subscribers[symbol]++

		if _, exists := client.subscriptions[symbol]; !exists {
			log.Printf("[DEBUG] No subscription for %s, creating one", symbol)
			go client.SubscribeToMarketData(symbol)
		} else {
			log.Printf("[DEBUG] Subscription exists for %s", symbol)
		}

		if quote, exists := client.quotes[symbol]; exists {
			if time.Since(quote.Timestamp) > 90*time.Second {
				quote.Stale = true
				log.Printf("[DEBUG] Quote for %s is stale", symbol)
			}

			log.Printf("[DEBUG] Found existing quote for %s: Bid=%.6f, Ask=%.6f, Last=%.6f", symbol, quote.Bid, quote.Ask, quote.Last)
			result[symbol] = &quote
		} else {
			if _, exists := client.quoteChannels[symbol]; !exists {
				client.quoteChannels[symbol] = make(chan struct{}, 1)
			}

			waitingSymbols = append(waitingSymbols, symbol)
			log.Printf("[DEBUG] No quote found for %s, will wait for first data", symbol)
		}
	}

	if len(waitingSymbols) > 0 {
		log.Printf("[DEBUG] Waiting for quotes for symbols: %v", waitingSymbols)

		timeoutCh := time.After(timeout)

		for _, symbol := range waitingSymbols {
			select {
			case <-client.quoteChannels[symbol]:
				// Проверяем, есть ли подписка - если нет, значит символ не найден
				if _, exists := client.subscriptions[symbol]; !exists {
					log.Printf("[DEBUG] Symbol %s not found (subscription removed)", symbol)
					result[symbol] = nil
				} else if quote, exists := client.quotes[symbol]; exists {
					log.Printf("[DEBUG] Received quote for %s: Bid=%.6f, Ask=%.6f, Last=%.6f", symbol, quote.Bid, quote.Ask, quote.Last)
					result[symbol] = &quote
				} else {
					log.Printf("[DEBUG] Channel notified but no quote found for %s", symbol)
					result[symbol] = nil
				}
			case <-timeoutCh:
				log.Printf("[DEBUG] Timeout waiting for quotes for %s", symbol)
				result[symbol] = nil
			}
		}
	}

	log.Printf("[DEBUG] Returning quotes with wait: %+v", result)
	return result
}

func (client *MarketDataClient) ReleaseQuotes(symbols []string) {
	for _, symbol := range symbols {
		if count, exists := client.subscribers[symbol]; exists && count > 0 {
			client.subscribers[symbol]--

			if client.subscribers[symbol] == 0 {
				go client.scheduleUnsubscribe(symbol)
			}
		}
	}
}

func (client *MarketDataClient) scheduleUnsubscribe(symbol string) {
	time.Sleep(60 * time.Second)

	if count, exists := client.subscribers[symbol]; exists && count == 0 {
		client.UnsubscribeFromMarketData(symbol)
		delete(client.subscribers, symbol)
	}
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}
