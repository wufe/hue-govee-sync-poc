package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type AuthResponse struct {
	Token string `json:"authentication_token"`
}

func main() {
	ip := "192.168.1.110" // Replace with your Twinkly IP
	authToken := login(ip)

	fmt.Println("Auth token:", authToken)

	turnOnLights(ip, authToken)
}

func login(ip string) string {
	url := fmt.Sprintf("http://%s/xled/v1/login", ip)
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"challenge":"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="}`))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var authResp AuthResponse
	json.NewDecoder(resp.Body).Decode(&authResp)

	return authResp.Token
}

func turnOnLights(ip, token string) {
	url := fmt.Sprintf("http://%s/xled/v1/led/mode", ip)

	data := map[string]string{"mode": "on"}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("Lights turned on:", resp.Status)
}

func turnOffLights(ip, token string) {
	url := fmt.Sprintf("http://%s/xled/v1/led/mode", ip)

	data := map[string]string{"mode": "off", "effect_id": "0"}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("Lights turned off:", resp.Status)
}
