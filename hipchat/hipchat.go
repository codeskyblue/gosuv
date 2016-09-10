package hipchat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const API = "https://api.hipchat.com/v2/room/%s/notification"

type apiResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type apiRequest struct {
	Message       string `json:"message"`
	MessageFormat string `json:"message_format"`
}

type configuration struct {
	accessToken string
	destination string
}

type Params struct {
	Token   string
	Room    string
	Title   string
	Message string
}

// Notify sends a message request to the HipChat API.
func Notify(n Params) error {
	payload := new(bytes.Buffer)
	err := json.NewEncoder(payload).Encode(apiRequest{
		Message:       fmt.Sprintf("%s\n%s", n.Title, n.Message),
		MessageFormat: "text",
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf(API, n.Room), payload)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.Token))
	req.Header.Set("Content-Type", "application/json")

	webClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := webClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decoding response: %s", err)
	}

	if m := r.Error.Message; m != "" {
		return fmt.Errorf("site: %s, msg: %v", "hipchat", m)
	}
	return nil
}
