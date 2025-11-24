package main

type TwinklyAuthResponse struct {
	Token                        string `json:"authentication_token"`
	AuthenticationTokenExpiresIn int    `json:"authentication_token_expires_in"`
}
