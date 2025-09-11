package orders

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

type OrdersClient struct {
	*bcb.BCBApplication
	initiator *quickfix.Initiator
	orders    map[string]*OrderInfo
}

type OrderInfo struct {
	ClOrdID     string
	Symbol      string
	Side        string
	OrderQty    float64
	Price       float64
	OrdType     string
	TimeInForce string
	Status      string
	ExecType    string
}

func NewOrdersClient() *OrdersClient {
	return &OrdersClient{
		BCBApplication: bcb.NewBCBApplication(),
		orders:         make(map[string]*OrderInfo),
	}
}

func (client *OrdersClient) Start(configFile string) error {
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

	log.Println("Orders client started successfully")
	return nil
}

func (client *OrdersClient) Stop() {
	if client.initiator != nil {
		client.initiator.Stop()
		log.Println("Orders client stopped")
	}
}

func (client *OrdersClient) NewOrderSingle(order *OrderInfo) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	message := quickfix.NewMessage()
	message.Body.SetString(tag.MsgType, "D")

	message.Body.SetString(tag.ClOrdID, order.ClOrdID)
	message.Body.SetString(tag.Symbol, order.Symbol)
	message.Body.SetString(tag.Side, order.Side)
	message.Body.SetString(tag.OrderQty, fmt.Sprintf("%.8f", order.OrderQty))
	message.Body.SetString(tag.OrdType, order.OrdType)
	message.Body.SetString(tag.TimeInForce, order.TimeInForce)
	message.Body.SetString(tag.TransactTime, time.Now().UTC().Format("20060102-15:04:05.000"))

	if order.OrdType == "2" && order.Price > 0 {
		message.Body.SetString(tag.Price, fmt.Sprintf("%.8f", order.Price))
	}

	message.Body.SetString(20030, "Y")

	if err := quickfix.SendToTarget(message, client.GetSessionID()); err != nil {
		return fmt.Errorf("failed to send order: %w", err)
	}

	client.orders[order.ClOrdID] = order

	log.Printf("New order sent: %s (%s %s %f @ %f)", order.ClOrdID, order.Side, order.Symbol, order.OrderQty, order.Price)
	return nil
}

func (client *OrdersClient) CancelOrder(origClOrdID, symbol, side string) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	newClOrdID := generateOrderID()
	message := quickfix.NewMessage()
	message.Body.SetString(tag.MsgType, "F")

	message.Body.SetString(tag.ClOrdID, newClOrdID)
	message.Body.SetString(tag.OrigClOrdID, origClOrdID)
	message.Body.SetString(tag.Symbol, symbol)
	message.Body.SetString(tag.Side, side)
	message.Body.SetString(tag.TransactTime, time.Now().UTC().Format("20060102-15:04:05.000"))

	if err := quickfix.SendToTarget(message, client.GetSessionID()); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	log.Printf("Cancel request sent for order: %s (new ClOrdID: %s)", origClOrdID, newClOrdID)
	return nil
}

func (client *OrdersClient) ReplaceOrder(origClOrdID string, newOrder *OrderInfo) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	message := quickfix.NewMessage()
	message.Body.SetString(tag.MsgType, "G") // Order Cancel/Replace Request

	message.Body.SetString(tag.ClOrdID, newOrder.ClOrdID)
	message.Body.SetString(tag.OrigClOrdID, origClOrdID)
	message.Body.SetString(tag.Symbol, newOrder.Symbol)
	message.Body.SetString(tag.Side, newOrder.Side)
	message.Body.SetString(tag.OrdType, newOrder.OrdType)
	message.Body.SetString(tag.OrderQty, fmt.Sprintf("%.8f", newOrder.OrderQty))
	message.Body.SetString(tag.TransactTime, time.Now().UTC().Format("20060102-15:04:05.000"))

	if newOrder.OrdType == "2" && newOrder.Price > 0 {
		message.Body.SetString(tag.Price, fmt.Sprintf("%.8f", newOrder.Price))
	}

	if err := quickfix.SendToTarget(message, client.GetSessionID()); err != nil {
		return fmt.Errorf("failed to replace order: %w", err)
	}

	client.orders[newOrder.ClOrdID] = newOrder

	log.Printf("Replace request sent for order: %s -> %s", origClOrdID, newOrder.ClOrdID)
	return nil
}

func (client *OrdersClient) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)

	switch msgType {
	case "8": // Execution Report
		client.handleExecutionReport(message)
	case "9": // Order Cancel Reject
		client.handleOrderCancelReject(message)
	default:
		return client.BCBApplication.FromApp(message, sessionID)
	}

	return nil
}

func (client *OrdersClient) handleExecutionReport(message *quickfix.Message) {
	clOrdID, _ := message.Body.GetString(tag.ClOrdID)
	orderID, _ := message.Body.GetString(tag.OrderID)
	execType, _ := message.Body.GetString(tag.ExecType)
	ordStatus, _ := message.Body.GetString(tag.OrdStatus)
	symbol, _ := message.Body.GetString(tag.Symbol)
	side, _ := message.Body.GetString(tag.Side)

	log.Printf("Execution Report: ClOrdID=%s, OrderID=%s, ExecType=%s, OrdStatus=%s, Symbol=%s, Side=%s",
		clOrdID, orderID, execType, ordStatus, symbol, side)

	if order, exists := client.orders[clOrdID]; exists {
		order.Status = ordStatus
		order.ExecType = execType
	}

	// TODO
}

func (client *OrdersClient) handleOrderCancelReject(message *quickfix.Message) {
	clOrdID, _ := message.Body.GetString(tag.ClOrdID)
	origClOrdID, _ := message.Body.GetString(tag.OrigClOrdID)
	cxlRejReason, _ := message.Body.GetString(tag.CxlRejReason)
	text, _ := message.Body.GetString(tag.Text)

	log.Printf("Order Cancel Reject: ClOrdID=%s, OrigClOrdID=%s, Reason=%s, Text=%s",
		clOrdID, origClOrdID, cxlRejReason, text)
}

func (client *OrdersClient) GetOrderStatus(clOrdID string) (*OrderInfo, bool) {
	order, exists := client.orders[clOrdID]
	return order, exists
}

func (client *OrdersClient) GetAllOrders() map[string]*OrderInfo {
	return client.orders
}

func generateOrderID() string {
	return fmt.Sprintf("ord-%d", time.Now().UnixNano())
}
