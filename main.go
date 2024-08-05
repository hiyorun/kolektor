package main

import (
	"flag"
	"kolektor/api"
	"kolektor/collector"
	"kolektor/config"
	"kolektor/db"
	"kolektor/store"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
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
	verboseLog := flag.Bool("v", false, "Enable debug logging")
	silentLog := flag.Bool("s", false, "Error only logging")
	flag.Parse()

	log.SetLevel(log.InfoLevel)
	if *verboseLog {
		log.SetLevel(log.DebugLevel)
	} else if *silentLog {
		log.SetLevel(log.ErrorLevel)
	}
	log.Debug("Loading config...")
	config, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal("Loading config", err)
	}

	log.Debug("Connecting to database...")
	db, err := db.Open(config)
	if err != nil {
		log.Fatal("Error opening db", err)
	}

	metrics := make(chan interface{})

	log.Debug("Starting store...")
	var store = store.NewSkuyliteStore(metrics, &config.Store, db)
	go store.Run()

	var collectors []collector.Collector
	log.Debug("Starting Kolektor...")
	for i, collectorCfg := range config.Collectors {
		collectorInstance, err := collector.CollectorFactory(collectorCfg, collectorCfg.Interval)
		if err != nil {
			log.Fatal("Error creating collector", err)
		}

		collectors = append(collectors, collectorInstance)
		go collectorInstance.Run(metrics)
		log.Infof("Started %s collector %d", collectorCfg.Type, i+1)
	}

	log.Infof("HTTP Server is listening on %s:%s", config.HTTP.Host, config.HTTP.Port)
	var httpServer = api.NewHTTPServer(config, db)
	go httpServer.Run()

	ListenSignal(func() {
		log.Debug("Stopping Kolektor...")
		for _, collectorInstance := range collectors {
			err := collectorInstance.Close()
			if err != nil {
				log.Fatal("Error closing collector", err)
			}
		}

		log.Debug("Stopping store...")
		err = store.Close()
		if err != nil {
			log.Fatal("Error closing store", err)
		}

		log.Debug("Stopping HTTP Server...")
		err = httpServer.Close()
		if err != nil {
			log.Fatal("Error closing http server", err)
		}

		log.Debug("Closing database...")
		err = db.Close()
		if err != nil {
			log.Fatal("Error closing db", err)
		}
		log.Info("Kolektor stopped")
	})
}
