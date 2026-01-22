package main

import (
	"flag"

	"github.com/bh90210/super/server/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	err := config.Init(*configPath)
	if err != nil {
		panic(err)
	}
}
