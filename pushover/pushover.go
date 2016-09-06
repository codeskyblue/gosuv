package pushover

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// API is the Pushover API endpoint.
	API = "https://api.pushover.net/1/messages.json"
)

type apiResponse struct {
	Info    string   `json:"info"`
	Status  int      `json:"status"`
	Request string   `json:"request"`
	Errors  []string `json:"errors"`
	Token   string   `json:"token"`
}

type Params struct {
	Token   string
	User    string
	Title   string
	Message string
}

// Notify sends a push request to the Pushover API.
func Notify(p Params) error {
	vals := make(url.Values)
	vals.Set("token", p.Token)
	vals.Set("user", p.User)
	vals.Set("message", p.Message)
	vals.Set("title", p.Title)

	log.Println(vals.Encode())
	webClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := webClient.PostForm(API, vals)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Println("posted")

	var r apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("decoding response: %s", err)
	}

	if r.Status != 1 {
		return errors.New(strings.Join(r.Errors, ": ")) //noti.APIError{Site: "Pushover", Msg: strings.Join(r.Errors, ": ")}
	} else if strings.Contains(r.Info, "no active devices") {
		return errors.New(r.Info) //noti.APIError{Site: "Pushover", Msg: r.Info}
	}

	return nil
}
