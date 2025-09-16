package bcb

import (
	"log"
	"time"

	"github.com/DmytroMania/project-unifin-fix-microservice/pkg/auth"
	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/tag"
)

type BCBApplication struct {
	sessionID quickfix.SessionID
	connected bool
	loggedIn  bool
}

func NewBCBApplication() *BCBApplication {
	return &BCBApplication{
		connected: false,
		loggedIn:  false,
	}
}

func (app *BCBApplication) OnCreate(sessionID quickfix.SessionID) {
	app.sessionID = sessionID
	app.connected = true
	log.Printf("[EVENT (SessionCreated)]: %s", sessionID)
}

func (app *BCBApplication) OnLogon(sessionID quickfix.SessionID) {
	app.loggedIn = true
	log.Printf("[EVENT (LogonSuccess)]: %s", sessionID)
}

func (app *BCBApplication) OnLogout(sessionID quickfix.SessionID) {
	app.loggedIn = false
	app.connected = false
	log.Printf("[EVENT (Logout)]: %s", sessionID)
}

func (app *BCBApplication) OnLogonError(sessionID quickfix.SessionID, err error) {
	app.loggedIn = false
	app.connected = false
	log.Printf("[EVENT (LogonError)]: %s - %v", sessionID, err)
}

func (app *BCBApplication) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {
	msgType, _ := message.Header.GetString(tag.MsgType)

	switch msgType {
	case "A":
		log.Printf("[SEND (LogonRequest)]: %s", sessionID)

		message.Body.SetInt(tag.HeartBtInt, 30)
		message.Body.SetBool(tag.ResetSeqNumFlag, true)
		message.Header.SetString(tag.SendingTime, time.Now().UTC().Format("20060102-15:04:05.000"))

		if err := auth.SignLogonMessage(message, sessionID); err != nil {
			log.Printf("[ERROR (LogonSigning)]: %v", err)
			return
		}

		log.Printf("[SEND (LogonRequestSent)]: %s", sessionID)
	default:
		// Убираем логи для других типов сообщений
	}
}

func (app *BCBApplication) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)

	switch msgType {
	case "A": // Logon response
		log.Printf("[RECEIVE (LogonResponse)]: %s", sessionID)
	case "5": // Logout
		log.Printf("[RECEIVE (Logout)]: %s", sessionID)
	}

	return nil
}

func (app *BCBApplication) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	// Убираем логи для исходящих приложений
	return nil
}

func (app *BCBApplication) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)

	switch msgType {
	case "W":
		app.handleMarketDataSnapshot(message, sessionID)
	case "8":
		app.handleExecutionReport(message, sessionID)
	case "Y":
		app.handleSecurityList(message, sessionID)
	}

	return nil
}

func (app *BCBApplication) handleMarketDataSnapshot(message *quickfix.Message, sessionID quickfix.SessionID) {
	symbol, _ := message.Body.GetString(tag.Symbol)
	log.Printf("[RECEIVE (MarketDataSnapshot)]: %s from %s", symbol, sessionID)

	// TODO: market data
}

func (app *BCBApplication) handleExecutionReport(message *quickfix.Message, sessionID quickfix.SessionID) {
	execType, _ := message.Body.GetString(tag.ExecType)
	ordStatus, _ := message.Body.GetString(tag.OrdStatus)
	log.Printf("[RECEIVE (ExecutionReport)]: ExecType=%s, OrdStatus=%s from %s", execType, ordStatus, sessionID)

	// TODO: execution reports
}

func (app *BCBApplication) handleSecurityList(message *quickfix.Message, sessionID quickfix.SessionID) {
	log.Printf("[RECEIVE (SecurityList)]: %s", sessionID)

	// TODO: security list
}

func (app *BCBApplication) IsConnected() bool {
	return app.connected
}

func (app *BCBApplication) IsLoggedIn() bool {
	return app.loggedIn
}

func (app *BCBApplication) GetSessionID() quickfix.SessionID {
	return app.sessionID
}
