package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/quickfixgo/quickfix"
	"github.com/quickfixgo/tag"
)

const (
	APIKey    = "BCBVQ5IAV8NF"
	APISecret = "n20h9lc1bir08r9meeu19qum00uyroik"
)

func CreateSignature(sendingTime string, seqNum int, senderCompID, targetCompID string) string {
	separator := "\001"
	data := sendingTime + separator + strconv.Itoa(seqNum) + separator + senderCompID + separator + targetCompID

	h := hmac.New(sha256.New, []byte(APISecret))
	h.Write([]byte(data))
	signature := h.Sum(nil)

	return base64.URLEncoding.EncodeToString(signature)
}

func SignLogonMessage(msg *quickfix.Message, sessionID quickfix.SessionID) error {
	sendingTime, err := msg.Body.GetString(tag.SendingTime)
	if err != nil {
		return fmt.Errorf("failed to get SendingTime: %w", err)
	}

	seqNum, err := msg.Body.GetInt(tag.MsgSeqNum)
	if err != nil {
		return fmt.Errorf("failed to get MsgSeqNum: %w", err)
	}

	senderCompID := sessionID.SenderCompID
	targetCompID := sessionID.TargetCompID

	signature := CreateSignature(sendingTime, seqNum, senderCompID, targetCompID)

	msg.Body.SetString(tag.Password, APIKey)
	msg.Body.SetInt(tag.RawDataLength, len(signature))
	msg.Body.SetString(tag.RawData, signature)

	return nil
}
