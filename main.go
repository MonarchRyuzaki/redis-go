package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/MonarchRyuzaki/redis-go/config"
	"github.com/MonarchRyuzaki/redis-go/server"
)

func setupFlags() {
	flag.StringVar(&config.Host, "host", "0.0.0.0", "host for the dice server")
	flag.IntVar(&config.Port, "port", 7379, "port for the dice server")
	flag.Parse()
}

func main() {
	setupFlags()
	log.Println("rolling the dice 🎲")

	var sigs chan os.Signal = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	
	var wg sync.WaitGroup
	wg.Add(2)
	
	go server.RunASyncTCPServer(&wg)
	go server.WaitForSignal(&wg, sigs)
	
	wg.Wait()
}
