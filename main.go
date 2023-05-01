package main

import (
	"log"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
)

type Settings struct {
	LMDBRootDirectory string `envconfig:"LMDB_ROOT_DIRECTORY" default:"."`
}

var s Settings

func main() {
	if err := envconfig.Process("", &s); err != nil {
		log.Fatalf("failed to read from env: %v", err)
		return
	}

	filepath.Join(s.LMDBRootDirectory, "")
}
