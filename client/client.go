package client

import (
	"net/http"
)

type Client struct {
	secret string
	id string

	client *http.Client
}

func New(clientId string, clientSecret string) *Client {
	return &Client{
		secret: clientSecret,
		id: clientId,
	}
}
