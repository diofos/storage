package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/anthdm/foreverstore/internal/api"
	"github.com/anthdm/foreverstore/internal/config"
	"github.com/anthdm/foreverstore/internal/node"
)

func main() {
	var (
		portFlag        = flag.String("port", "7001", "TCP peer listen port")
		apiPortFlag     = flag.String("api", "8070", "HTTP REST API listen port")
		dataDirFlag     = flag.String("data", "", "Data directory (default ./meshstore_data_<port>)")
		nameFlag        = flag.String("name", "", "Node display name")
		peersFlag       = flag.String("peers", "", "Comma-separated list of bootstrap peers (e.g. 127.0.0.1:7001)")
		maxGBFlag       = flag.Int64("max-gb", 10, "Maximum contributed storage in GB")
		repFactorFlag   = flag.Int("replication", 2, "Default replication factor")
		chunkMBFlag     = flag.Int("chunk-mb", 1, "Chunk size in MB")
		configPathFlag  = flag.String("config", "", "Path to JSON configuration file")
	)
	flag.Parse()

	var cfg *config.Config
	var err error

	if *configPathFlag != "" {
		cfg, err = config.LoadConfig(*configPathFlag)
		if err != nil {
			log.Fatalf("failed to load config from %s: %v", *configPathFlag, err)
		}
	} else {
		cfg = config.DefaultConfig(*portFlag)
		cfg.Node.ListenAddress = "0.0.0.0:" + *portFlag
		cfg.API.ListenAddress = "127.0.0.1:" + *apiPortFlag
		if *dataDirFlag != "" {
			cfg.Node.DataDirectory = *dataDirFlag
		}
		if *nameFlag != "" {
			cfg.Node.DisplayName = *nameFlag
		}
		if *peersFlag != "" {
			cfg.Mesh.Peers = strings.Split(*peersFlag, ",")
		}
		cfg.Node.MaxStorageGB = *maxGBFlag
		cfg.Storage.ReplicationFactor = *repFactorFlag
		cfg.Storage.ChunkSizeMB = *chunkMBFlag
	}

	fmt.Println("==================================================")
	fmt.Printf("Starting MeshStore Node: %s\n", cfg.Node.DisplayName)
	fmt.Printf("P2P Listen Address:      %s\n", cfg.Node.ListenAddress)
	fmt.Printf("REST API Address:        http://%s\n", cfg.API.ListenAddress)
	fmt.Printf("Data Directory:          %s\n", cfg.Node.DataDirectory)
	fmt.Printf("Max Storage:             %d GB\n", cfg.Node.MaxStorageGB)
	fmt.Printf("Replication Factor:      %d\n", cfg.Storage.ReplicationFactor)
	fmt.Printf("Bootstrap Peers:         %v\n", cfg.Mesh.Peers)
	fmt.Println("==================================================")

	agent, err := node.NewNode(cfg)
	if err != nil {
		log.Fatalf("failed to initialize node agent: %v", err)
	}

	if err := agent.Start(); err != nil {
		log.Fatalf("failed to start node agent: %v", err)
	}

	apiServer := api.NewServer(agent)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server stopped: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down MeshStore...")
	_ = apiServer.Stop()
	agent.Stop()
	fmt.Println("MeshStore shutdown complete.")
}
