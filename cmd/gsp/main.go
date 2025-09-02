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

func runCli(commands *app.CliCommands) error {
	cmd := os.Args[1]
	var args []string

	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	if err := commands.Run(cmd, args...); err != nil {
		return err
	}

	return nil

}

func main() {
	if err := loadEnv(); err != nil {
		log.Fatal(err)
	}

	a := app.New(clientId, clientSecret, redirectUri)

	if err := a.Setup(); err != nil {
		log.Fatal(err)
	}

	cli := app.NewCliCommands(a)

	if len(os.Args) == 1 {
		if err := a.Run(); err != nil {
			log.Fatal(err)
		}
		return
	} else {
		if err := runCli(cli); err != nil {
			log.Fatal(err)
		}
	}



}
