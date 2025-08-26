package main

import (
	"go-spotify/app"
	"github.com/joho/godotenv"
	"log"
	"os"
)

var clientSecret string
var clientId string
var redirectUri string

func loadEnv() error {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	clientId = os.Getenv("CLIENTID")
	clientSecret = os.Getenv("CLIENTSECRET")
	redirectUri = os.Getenv("REDIRECTURI")

	return nil
}

func main() {
	if err := loadEnv(); err != nil {
		log.Fatal(err)
	}

	app := app.New(clientId, clientSecret, redirectUri)

	if err := app.Setup(); err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
