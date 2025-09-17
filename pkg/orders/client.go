package orders

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"bcb-fix-microservice/pkg/bcb"
	"bcb-fix-microservice/pkg/logging"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/tag"
)

type OrdersClient struct {
	*bcb.BCBApplication
	initiator  *quickfix.Initiator
	orders     map[string]*OrderInfo
	executions map[string][]*ExecutionInfo
}

type OrderInfo struct {
	ClOrdID      string    `json:"cl_ord_id"`
	OrderID      string    `json:"order_id"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	OrderQty     float64   `json:"order_qty"`
	Price        float64   `json:"price"`
	OrdType      string    `json:"ord_type"`
	TimeInForce  string    `json:"time_in_force"`
	Status       string    `json:"status"`
	ExecType     string    `json:"exec_type"`
	CumQty       float64   `json:"cum_qty"`
	LeavesQty    float64   `json:"leaves_qty"`
	AvgPx        float64   `json:"avg_px"`
	LastPx       float64   `json:"last_px"`
	LastQty      float64   `json:"last_qty"`
	Commission   float64   `json:"commission"`
	TransactTime time.Time `json:"transact_time"`
	LastExecTime time.Time `json:"last_exec_time"`
	RejectReason string    `json:"reject_reason"`
}

type ExecutionInfo struct {
	ClOrdID    string    `json:"cl_ord_id"`
	OrderID    string    `json:"order_id"`
	ExecID     string    `json:"exec_id"`
	ExecType   string    `json:"exec_type"`
	OrdStatus  string    `json:"ord_status"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	ExecQty    float64   `json:"exec_qty"`
	ExecPrice  float64   `json:"exec_price"`
	LeavesQty  float64   `json:"leaves_qty"`
	CumQty     float64   `json:"cum_qty"`
	AvgPx      float64   `json:"avg_px"`
	Commission float64   `json:"commission"`
	ExecTime   time.Time `json:"exec_time"`
	Text       string    `json:"text"`
}

func NewOrdersClient() *OrdersClient {
	return &OrdersClient{
		BCBApplication: bcb.NewBCBApplication(),
		orders:         make(map[string]*OrderInfo),
		executions:     make(map[string][]*ExecutionInfo),
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

	client.BCBApplication.SetInitiator(client.initiator)

	if err := client.initiator.Start(); err != nil {
		return fmt.Errorf("failed to start initiator: %w", err)
	}

	log.Println("[EVENT (OrdersClientStarted)]")
	return nil
}

func (client *OrdersClient) Stop() {
	if client.initiator != nil {
		client.initiator.Stop()

		log.Println("[EVENT (OrdersClientStopped)]")
	}
}

func (client *OrdersClient) NewOrderSingle(order *OrderInfo) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	message := quickfix.NewMessage()
	message.Header.SetString(tag.MsgType, "D")

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

	order.TransactTime = time.Now()
	order.LeavesQty = order.OrderQty

	client.orders[order.ClOrdID] = order

	log.Printf("[SEND (NewOrder)]: %s (%s %s %f @ %f)", order.ClOrdID, order.Side, order.Symbol, order.OrderQty, order.Price)
	return nil
}

func (client *OrdersClient) CancelOrder(origClOrdID, symbol, side string) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	newClOrdID := generateOrderID()
	message := quickfix.NewMessage()
	message.Header.SetString(tag.MsgType, "F")

	message.Body.SetString(tag.ClOrdID, newClOrdID)
	message.Body.SetString(tag.OrigClOrdID, origClOrdID)
	message.Body.SetString(tag.Symbol, symbol)
	message.Body.SetString(tag.Side, side)
	message.Body.SetString(tag.TransactTime, time.Now().UTC().Format("20060102-15:04:05.000"))

	if err := quickfix.SendToTarget(message, client.GetSessionID()); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	log.Printf("[SEND (CancelOrder)]: %s (new ClOrdID: %s)", origClOrdID, newClOrdID)
	return nil
}

func (client *OrdersClient) ReplaceOrder(origClOrdID string, newOrder *OrderInfo) error {
	if !client.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	message := quickfix.NewMessage()
	message.Header.SetString(tag.MsgType, "G")

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

	log.Printf("[SEND (ReplaceOrder)]: %s -> %s", origClOrdID, newOrder.ClOrdID)
	return nil
}

func (client *OrdersClient) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Header.GetString(tag.MsgType)

	switch msgType {
	case "8":
		client.handleExecutionReport(message)
	case "9":
		client.handleOrderCancelReject(message)
	default:
		return client.BCBApplication.FromApp(message, sessionID)
	}

	return nil
}

func (client *OrdersClient) handleExecutionReport(message *quickfix.Message) {
	clOrdID, _ := message.Body.GetString(tag.ClOrdID)
	orderID, _ := message.Body.GetString(tag.OrderID)
	execID, _ := message.Body.GetString(tag.ExecID)
	execType, _ := message.Body.GetString(tag.ExecType)
	ordStatus, _ := message.Body.GetString(tag.OrdStatus)
	symbol, _ := message.Body.GetString(tag.Symbol)
	side, _ := message.Body.GetString(tag.Side)
	text, _ := message.Body.GetString(tag.Text)

	lastQtyStr, _ := message.Body.GetString(tag.LastQty)
	lastPxStr, _ := message.Body.GetString(tag.LastPx)
	leavesQtyStr, _ := message.Body.GetString(tag.LeavesQty)
	cumQtyStr, _ := message.Body.GetString(tag.CumQty)
	avgPxStr, _ := message.Body.GetString(tag.AvgPx)
	commissionStr, _ := message.Body.GetString(tag.Commission)

	lastQty, _ := strconv.ParseFloat(lastQtyStr, 64)
	lastPx, _ := strconv.ParseFloat(lastPxStr, 64)
	leavesQty, _ := strconv.ParseFloat(leavesQtyStr, 64)
	cumQty, _ := strconv.ParseFloat(cumQtyStr, 64)
	avgPx, _ := strconv.ParseFloat(avgPxStr, 64)
	commission, _ := strconv.ParseFloat(commissionStr, 64)

	transactTimeStr, _ := message.Body.GetString(tag.TransactTime)
	execTime, _ := time.Parse("20060102-15:04:05.000", transactTimeStr)

	log.Printf("[RECEIVE (ExecutionReport)]: ClOrdID=%s, OrderID=%s, ExecType=%s, OrdStatus=%s, Symbol=%s, Side=%s, LastQty=%.6f, LastPx=%.6f, CumQty=%.6f, LeavesQty=%.6f",
		clOrdID, orderID, execType, ordStatus, symbol, side, lastQty, lastPx, cumQty, leavesQty)

	execution := &ExecutionInfo{
		ClOrdID:    clOrdID,
		OrderID:    orderID,
		ExecID:     execID,
		ExecType:   execType,
		OrdStatus:  ordStatus,
		Symbol:     symbol,
		Side:       side,
		ExecQty:    lastQty,
		ExecPrice:  lastPx,
		LeavesQty:  leavesQty,
		CumQty:     cumQty,
		AvgPx:      avgPx,
		Commission: commission,
		ExecTime:   execTime,
		Text:       text,
	}

	client.executions[clOrdID] = append(client.executions[clOrdID], execution)

	if order, exists := client.orders[clOrdID]; exists {
		order.OrderID = orderID
		order.Status = ordStatus
		order.ExecType = execType
		order.CumQty = cumQty
		order.LeavesQty = leavesQty
		order.AvgPx = avgPx
		order.LastPx = lastPx
		order.LastQty = lastQty
		order.Commission += commission
		order.LastExecTime = execTime

		/* 8 - rejected */
		if ordStatus == "8" {
			order.RejectReason = text
		}

		log.Printf("[UPDATE (Order)]: %s - Status=%s, CumQty=%.6f, LeavesQty=%.6f, AvgPx=%.6f, Commission=%.6f",
			clOrdID, ordStatus, cumQty, leavesQty, avgPx, order.Commission)
	}
}

func (client *OrdersClient) handleOrderCancelReject(message *quickfix.Message) {
	clOrdID, _ := message.Body.GetString(tag.ClOrdID)
	origClOrdID, _ := message.Body.GetString(tag.OrigClOrdID)
	cxlRejReason, _ := message.Body.GetString(tag.CxlRejReason)
	text, _ := message.Body.GetString(tag.Text)

	log.Printf("[RECEIVE (OrderCancelReject)]: ClOrdID=%s, OrigClOrdID=%s, Reason=%s, Text=%s",
		clOrdID, origClOrdID, cxlRejReason, text)
}

func (client *OrdersClient) GetOrderStatus(clOrdID string) (*OrderInfo, bool) {
	order, exists := client.orders[clOrdID]
	return order, exists
}

func (client *OrdersClient) GetAllOrders() map[string]*OrderInfo {
	return client.orders
}

func (client *OrdersClient) GetOrderExecutions(clOrdID string) ([]*ExecutionInfo, bool) {
	executions, exists := client.executions[clOrdID]
	return executions, exists
}

func (client *OrdersClient) GetAllExecutions() map[string][]*ExecutionInfo {
	return client.executions
}

func (client *OrdersClient) GetConnectionStatus() map[string]interface{} {
	return client.BCBApplication.GetConnectionStatus()
}

func generateOrderID() string {
	return fmt.Sprintf("ord-%d", time.Now().UnixNano())
}
