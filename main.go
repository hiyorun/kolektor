package main

import (
	"flag"
	"kolektor/api"
	"kolektor/collector"
	"kolektor/config"
	"kolektor/db"
	"kolektor/store"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func ListenSignal(interrupt func()) {
	signals := []os.Signal{syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGSTOP, os.Interrupt}
	signalSink := make(chan os.Signal, 1)

	defer close(signalSink)

	signal.Notify(signalSink, signals...)
	<-signalSink

	interrupt()
}

func main() {
	cfgPath := flag.String("c", "./config.yaml", "Path to YAML configuration file")
	flag.Parse()

	log.Println("Loading config...")
	config, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Connecting to database...")
	db, err := db.Open(config)
	if err != nil {
		log.Fatal("Error opening db:", err.Error())
	}

	metrics := make(chan interface{})

	log.Println("Starting store...")
	var store = store.NewSkuyliteStore(metrics, &config.Store, db)
	go store.Run()

	var collectors []collector.Collector
	log.Println("Starting Kolektor...")
	for i, collectorCfg := range config.Collectors {
		collectorInstance, err := collector.CollectorFactory(collectorCfg, collectorCfg.Interval)
		if err != nil {
			log.Fatal(err.Error())
		}

		collectors = append(collectors, collectorInstance)
		go collectorInstance.Run(metrics)
		log.Printf("Started %s collector %d", collectorCfg.Type, i+1)
	}

	log.Printf("HTTP Server is listening on %s:%s", config.HTTP.Host, config.HTTP.Port)
	var httpServer = api.NewHTTPServer(config, db)
	go httpServer.Run()

	ListenSignal(func() {
		log.Println("Stopping Kolektor...")
		for _, collectorInstance := range collectors {
			collectorInstance.Close()
		}

		log.Println("Stopping store...")
		store.Close()

		log.Println("Stopping HTTP Server...")
		httpServer.Close()

		log.Println("Closing database...")
		err := db.Close()
		if err != nil {
			log.Fatal("Error closing db:", err.Error())
		}
	})
}
