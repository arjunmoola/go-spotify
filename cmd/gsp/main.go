package main

import (
	"github.com/arjunmoola/go-spotify/app"
	"github.com/arjunmoola/go-spotify/utils"

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
	if err := utils.InitializeConfigDir(); err != nil {
		log.Fatal(err)
	}

	logFile, err := utils.OpenLogFile()

	if err != nil {
		log.Fatal(err)
	}

	defer logFile.Close()

	logger := utils.NewLogger(logFile)
	app.SetupLogger(logger)

	db, err := utils.InitializeDB()

	if err != nil {
		log.Fatal(err)
	}

	a := app.New(db)

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
