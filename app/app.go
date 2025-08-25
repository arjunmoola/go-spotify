package app

import (
	//tea "github.com/charmbracelet/bubbletea"
	"log"
	"context"
	"database/sql"
	schema "go-spotify/sql"
	"path/filepath"
	"go-spotify/database"
	_ "github.com/tursodatabase/go-libsql"
	"errors"
	//"log"
	"os"
)

var debug bool = true
var ddl string

func init() {
	ddl = schema.Get()
}

const (
	defaultConfigDirName = ".go-spotify"
	defaultDbName = "go-spotify.db"
	dbDriver = "libsql"
)

var (
	ErrInvalidClientInfo = errors.New("either the client token or secret is incorrect")
	ErrClientInfoNotFound = errors.New("client info could not be found")
)

func getDbUrl(configDir string) string {
	return filepath.Join(configDir, defaultDbName)
}

func getConfigDir() string {
	userHome, _ := os.UserHomeDir()
	return filepath.Join(userHome, defaultConfigDirName)
}

func (a *App) initializeConfigDir() error {
	_, err := os.Lstat(a.configDir)

	var dirNotFound bool
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			dirNotFound = true
		} else {
			return err
		}
	}

	if dirNotFound {
		return os.Mkdir(a.configDir, 0777)
	}

	return nil
}

func (a *App) initializeDb() error {
	dburl := "file:" + filepath.Join(a.configDir, defaultDbName)

	db, err := sql.Open(dbDriver, dburl)

	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	_, err = db.Exec(ddl)

	if err != nil {
		return err
	}

	a.db = db
	a.dbUrl = dburl

	return nil
}

func (a *App) getClientInfo() error {
	queries := database.New(a.db)

	var clientInfoNotFound bool

	row, err := queries.GetClientInfo(context.Background())

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			clientInfoNotFound = true
		} else {
			return err
		}
	}

	if clientInfoNotFound {
		params := database.InsertConfigParams{
			ClientID: a.clientId,
			ClientSecret: a.clientSecret,
		}

		if err := queries.InsertConfig(context.Background(), params); err != nil {
			return err
		}
	} else {
		log.Println("client info has been found in database")
	}

	a.clientSecret = row.ClientSecret
	a.clientId = row.ClientID
	log.Printf("clientId: %s, client secret: %s", a.clientId, a.clientSecret)
	return nil

}


type App struct {
	clientId string
	clientSecret string

	db *sql.DB
	configDir string
	dbUrl string
}

func New(clientId, clientSecret string) *App {
	dir := getConfigDir()
	dburl := getDbUrl(dir)


	return &App{
		clientId: clientId,
		clientSecret: clientSecret,
		dbUrl: dburl,
		configDir: dir,
	}
}

func (a *App) Init() error {
	if err := a.initializeConfigDir(); err != nil {
		return err
	}

	if err := a.initializeDb(); err != nil {
		return err
	}

	if err := a.getClientInfo(); err != nil {
		return err
	}

	return nil
}
