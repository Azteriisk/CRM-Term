package main

import (
	"context"
	"log"
	"os"

	"crmterm/internal/config"
	"crmterm/internal/storage"
	"crmterm/internal/ui"
)

func main() {
	ctx := context.Background()

	cfgStore, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := storage.Open(ctx)
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	defer db.Close()

	program := ui.NewProgram(db, cfgStore)
	if err := program.Start(); err != nil {
		log.Println("program terminated:", err)
		os.Exit(1)
	}
}
