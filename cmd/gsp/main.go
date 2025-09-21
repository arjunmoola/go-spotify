package main

import (
	"github.com/arjunmoola/go-spotify/app"
	"log"
	"os"
)

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
	a := app.New()

	if err := a.Setup(); err != nil {
		log.Fatal(err)
	}

	cli := app.NewCliCommands(a)

	if len(os.Args) == 1 {
		if err := a.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := a.SetupCli(); err != nil {
			log.Fatal(err)
		}

		if err := runCli(cli); err != nil {
			log.Fatal(err)
		}
	}



}
