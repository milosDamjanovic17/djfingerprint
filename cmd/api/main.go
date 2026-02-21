package main

import (
	"fmt"
	"log"

	"github.com/milosDamjanovic17/djfingerprint/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to laod the config: %v", err)
	}

	fmt.Printf("DJ Fingeprint API starting on port %s\n", cfg.AppPort)
	fmt.Printf("Environemnt: %s\n", cfg.AppEnv)
	fmt.Printf("Database: %s@%s/%s\n", cfg.DBUser, cfg.DBHost, cfg.DBName)
}