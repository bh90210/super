package super

import (
	"os"
	"path/filepath"
)

const SuperServer = "localhost:80"

// const SuperServer = "super.aeroponics.club:80"

type Storage string

const superStore Storage = ".super"

const SearchStore Storage = "search"

const DataStore Storage = "db"

const MusicStore Storage = "music"

func LocalStorage(store ...Storage) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	paths := []string{homeDir, string(superStore)}
	for _, s := range store {
		paths = append(paths, string(s))
	}

	return filepath.Join(paths...)
}

const (
	File = "file_"
)
