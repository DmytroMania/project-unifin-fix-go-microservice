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
	log.Printf("Session created: %s", sessionID)
	log.Printf("SenderCompID: %s, TargetCompID: %s", sessionID.SenderCompID, sessionID.TargetCompID)
	log.Printf("BeginString: %s", sessionID.BeginString)
}

func (app *BCBApplication) OnLogon(sessionID quickfix.SessionID) {
	app.loggedIn = true
	log.Printf("Logged on: %s", sessionID)
}

func (app *BCBApplication) OnLogout(sessionID quickfix.SessionID) {
	app.loggedIn = false
	app.connected = false
	log.Printf("Logged out: %s", sessionID)
}

func (app *BCBApplication) OnLogonError(sessionID quickfix.SessionID, err error) {
	app.loggedIn = false
	app.connected = false
	log.Printf("Logon error for %s: %v", sessionID, err)
}

func (app *BCBApplication) ToAdmin(message *quickfix.Message, sessionID quickfix.SessionID) {
	msgType, _ := message.Header.GetString(tag.MsgType)
	log.Printf("ToAdmin called for message type: %s, session: %s", msgType, sessionID)

	switch msgType {
	case "A":
		log.Printf("Processing LOGON message for session: %s", sessionID)
		log.Printf("Original message: %s", message.String())

		message.Body.SetInt(tag.HeartBtInt, 30)
		message.Body.SetBool(tag.ResetSeqNumFlag, true)
		message.Header.SetString(tag.SendingTime, time.Now().UTC().Format("20060102-15:04:05.000"))

		log.Printf("Message after basic fields: %s", message.String())

		if err := auth.SignLogonMessage(message, sessionID); err != nil {
			log.Printf("Error signing logon message: %v", err)
			return
		}

		log.Printf("Final signed message: %s", message.String())
		log.Printf("Logon message prepared for %s", sessionID)
	default:
		log.Printf("ToAdmin: Other admin message type: %s", msgType)
	}
}

func (app *BCBApplication) FromAdmin(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)
	log.Printf("FromAdmin: Received admin message type %s from %s", msgType, sessionID)
	log.Printf("FromAdmin message: %s", message.String())

	switch msgType {
	case "A": // Logon response
		log.Printf("Received LOGON response from %s", sessionID)
	case "5": // Logout
		log.Printf("Received LOGOUT from %s", sessionID)
	}

	return nil
}

func (app *BCBApplication) ToApp(message *quickfix.Message, sessionID quickfix.SessionID) error {
	log.Printf("Sending app message to %s", sessionID)
	return nil
}

func (app *BCBApplication) FromApp(message *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	msgType, _ := message.Body.GetString(tag.MsgType)
	log.Printf("Received app message type %s from %s", msgType, sessionID)

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
	log.Printf("Market data snapshot for %s received from %s", symbol, sessionID)

	// TODO: market data
}

func (app *BCBApplication) handleExecutionReport(message *quickfix.Message, sessionID quickfix.SessionID) {
	execType, _ := message.Body.GetString(tag.ExecType)
	ordStatus, _ := message.Body.GetString(tag.OrdStatus)
	log.Printf("Execution report: ExecType=%s, OrdStatus=%s from %s", execType, ordStatus, sessionID)

	// TODO: execution reports
}

func (app *BCBApplication) handleSecurityList(message *quickfix.Message, sessionID quickfix.SessionID) {
	log.Printf("Security list received from %s", sessionID)

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
