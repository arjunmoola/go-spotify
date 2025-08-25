package main

import (
	"go-spotify/app"
	"github.com/joho/godotenv"
	"log"
	"os"
)

var clientSecret string
var clientId string

func loadEnv() error {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	clientId = os.Getenv("CLIENTID")
	clientSecret = os.Getenv("CLIENTSECRET")

	return nil
}

func main() {
	if err := loadEnv(); err != nil {
		log.Fatal(err)
	}

	app := app.New(clientId, clientSecret)

	if err := app.Init(); err != nil {
		log.Fatal(err)
	}

}
