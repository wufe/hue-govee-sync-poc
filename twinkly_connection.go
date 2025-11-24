package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

type TwinklyCommandSender interface {
	SendMsg(message TwinklyMessage) error
}

type TwinklyConnection struct {
	ip             string
	authToken      atomic.String
	pollingStarted bool
}

func NewTwinklyConnection(ip string) *TwinklyConnection {
	return &TwinklyConnection{ip: ip}
}

func NewNoopTwinklyConnection() *TwinklyConnection {
	return &TwinklyConnection{}
}

var _ TwinklyCommandSender = (*TwinklyConnection)(nil)

func (c *TwinklyConnection) Login(ctx context.Context, ip string) error {
	if c.ip == "" {
		return nil
	}

	url := fmt.Sprintf("http://%s/xled/v1/login", ip)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(`{"challenge":"test"}`))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending login request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status code %d", resp.StatusCode)
	}

	var authResp TwinklyAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("error decoding login response: %v", err)
	}

	verifyUrl := fmt.Sprintf("http://%s/xled/v1/verify", ip)
	verifyReq, _ := http.NewRequestWithContext(ctx, "POST", verifyUrl, nil)
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyReq.Header.Set("X-Auth-Token", authResp.Token)

	verifyResp, err := client.Do(verifyReq)
	if err != nil {
		return fmt.Errorf("error sending verify request: %v", err)
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode != http.StatusOK {
		fmt.Println("Verification failed:", verifyResp.Status)
		return fmt.Errorf("verification failed with status code %d", verifyResp.StatusCode)
	}

	c.authToken.Store(authResp.Token)

	if !c.pollingStarted {
		c.pollingStarted = true
		c.startLoginPolling(ctx, authResp.AuthenticationTokenExpiresIn)
	}

	return nil
}

func (c *TwinklyConnection) startLoginPolling(ctx context.Context, expirationInSeconds int) {
	if c.ip == "" {
		return
	}

	go func() {
		sleep := (time.Duration(expirationInSeconds) / 2) * time.Millisecond
		for ctx.Err() == nil {
			time.Sleep(sleep)
			err := c.Login(ctx, c.ip)
			if err != nil {
				log.Err(err).Msgf("error logging in and verifying twinkly connection: %s", err)
				sleep = 1 * time.Second
				continue
			}
			sleep = (time.Duration(expirationInSeconds) / 2) * time.Millisecond
		}
	}()
}

func (c *TwinklyConnection) turnOn(ctx context.Context) error {
	if c.ip == "" {
		return nil
	}

	data := map[string]string{"mode": "movie"}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling turn on request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/xled/v1/led/mode", c.ip), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating turn on request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", c.authToken.Load())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending turn on request: %v", err)
	}
	defer resp.Body.Close()

	// data = map[string]string{}
	// jsonData, err = json.Marshal(data)
	// if err != nil {
	// 	return fmt.Errorf("error marshalling turn on request: %v", err)
	// }

	// req, err = http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://%s/xled/v1/led/movie/config", c.ip), bytes.NewBuffer(jsonData))
	// if err != nil {
	// 	return fmt.Errorf("error creating turn on request: %v", err)
	// }
	// req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("X-Auth-Token", c.authToken.Load())

	// client := &http.Client{}
	// resp, err := client.Do(req)
	// if err != nil {
	// 	return fmt.Errorf("error sending turn on request: %v", err)
	// }
	// defer resp.Body.Close()

	return nil
}

func (c *TwinklyConnection) turnOff(ctx context.Context) error {
	if c.ip == "" {
		return nil
	}

	url := fmt.Sprintf("http://%s/xled/v1/led/mode", c.ip)

	data := map[string]string{"mode": "off"}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling turn off request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating turn off request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", c.authToken.Load())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending turn off request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *TwinklyConnection) SendMsg(message TwinklyMessage) error {
	if c.ip == "" {
		return nil
	}

	switch message {
	case TwinklyMessageOn:
		log.Debug().Msg("Turning on Twinkly lights")
		return c.turnOn(context.Background())
	case TwinklyMessageOff:
		log.Debug().Msg("Turning off Twinkly lights")
		return c.turnOff(context.Background())
	default:
		return fmt.Errorf("unknown Twinkly message: %s", message)
	}
}
