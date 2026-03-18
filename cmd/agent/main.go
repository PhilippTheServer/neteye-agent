package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/neteye/agent/internal/client"
	"github.com/neteye/agent/internal/config"
)

func main() {
	cfgPath := flag.String("config", "", "path to YAML config file (optional)")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[neteye-agent] ")

	log.Printf("starting | hostname=%s os=%s arch=%s center=%s interval=%v",
		cfg.Hostname, config.OS(), config.Arch(), cfg.CenterURL, cfg.CollectInterval)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := client.New(cfg)
	c.Run(ctx)

	log.Println("stopped")
}
