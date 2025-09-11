package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/quickfixgo/quickfix"
)

type DebugLogFactory struct {
	logPath string
}

func NewDebugLogFactory(logPath string) *DebugLogFactory {
	if err := os.MkdirAll(logPath, 0755); err != nil {
		log.Printf("Warning: Failed to create log directory %s: %v", logPath, err)
	}

	return &DebugLogFactory{logPath: logPath}
}

func (f *DebugLogFactory) Create() (quickfix.Log, error) {
	return f.createLog("global")
}

func (f *DebugLogFactory) CreateSessionLog(sessionID quickfix.SessionID) (quickfix.Log, error) {
	sessionName := fmt.Sprintf("%s_%s_%s",
		sessionID.BeginString,
		sessionID.SenderCompID,
		sessionID.TargetCompID)
	return f.createLog(sessionName)
}

func (f *DebugLogFactory) createLog(name string) (quickfix.Log, error) {
	fileName := filepath.Join(f.logPath, fmt.Sprintf("%s_%s.log", name, time.Now().Format("20060102_150405")))
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %s: %w", fileName, err)
	}

	return NewDebugLog(file, name), nil
}

type DebugLog struct {
	file   *os.File
	name   string
	logger *log.Logger
}

func NewDebugLog(file *os.File, name string) *DebugLog {
	return &DebugLog{
		file:   file,
		name:   name,
		logger: log.New(file, fmt.Sprintf("[%s] ", name), log.LstdFlags|log.Lmicroseconds),
	}
}

func (l *DebugLog) OnIncoming(data []byte) {
	l.logger.Printf("INCOMING: %s", string(data))
	fmt.Printf("[%s] INCOMING: %s\n", l.name, string(data))
}

func (l *DebugLog) OnOutgoing(data []byte) {
	l.logger.Printf("OUTGOING: %s", string(data))
	fmt.Printf("[%s] OUTGOING: %s\n", l.name, string(data))
}

func (l *DebugLog) OnEvent(msg string) {
	l.logger.Printf("EVENT: %s", msg)
	fmt.Printf("[%s] EVENT: %s\n", l.name, msg)
}

func (l *DebugLog) OnEventf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.OnEvent(msg)
}

func (l *DebugLog) OnErrorEvent(msg string) {
	l.logger.Printf("ERROR: %s", msg)
	fmt.Printf("[%s] ERROR: %s\n", l.name, msg)
}

func (l *DebugLog) OnErrorEventf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.OnErrorEvent(msg)
}

func (l *DebugLog) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
