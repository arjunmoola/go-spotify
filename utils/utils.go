package utils

import (
	"path/filepath"
	"errors"
	"os"
	"log/slog"
	"database/sql"
	"io"
	_ "github.com/tursodatabase/go-libsql"
	schema "github.com/arjunmoola/go-spotify/sql"
)

var configDirName = ".go-spotify"
var defaultDbName = "go-spotify.db"
var userHomeDir string
var configDir string
var dbUrl string
var logFilePath string

var ddl string

func init() {
	homeDir, _ := os.UserHomeDir()
	userHomeDir = homeDir
	configDir = filepath.Join(homeDir, configDirName)
	dbUrl = "file:" + filepath.Join(configDir, defaultDbName)
	logFilePath = filepath.Join(configDir, "log")

	ddl = schema.Get()
}

func UserHomeDir() string {
	return userHomeDir
}

func ConfigDir() string {
	return configDir
}

func DbUrl() string {
	return dbUrl
}

func LogFilePath() string {
	return logFilePath
}

func InitializeConfigDir() error {
	userHomeDir, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	configDir := filepath.Join(userHomeDir, ".go-spotify")

	if err := checkOrCreateDir(configDir); err != nil {
		return err
	}

	return nil
}

func InitializeDB() (*sql.DB, error) {
	dbUrl := DbUrl()

	db, err := sql.Open("libsql", dbUrl)

	if err != nil {
		return nil,err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func OpenLogFile() (*os.File, error) {
	logPath := LogFilePath()
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		return nil, err
	}

	return file, nil
}

func NewLogger(w io.Writer) *slog.Logger {
	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{ Level: slog.LevelDebug, AddSource: false}))
	
	return logger
}

func checkOrCreateDir(dir string) error {
	var dirNotFound bool

	_, err := os.Lstat(dir)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			dirNotFound = true
		} else {
			return err
		}
	}

	if dirNotFound {
		if err := os.Mkdir(dir, 0766); err != nil {
			return err
		}
	}

	return nil
}
