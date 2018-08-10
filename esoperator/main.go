package main

import (
	"net/http"
	"net"
	"github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/signal"
	"syscall"
	"sync"
)

func main(){
	doneChan := make(chan struct{})
	var wg sync.WaitGroup
	
	r := prometheus.NewRegistry()
	r.MustRegister(prometheus.NewProcessCollector(os.Getpid(), ""))
	r.MustRegister(prometheus.NewGoCollector())
	mux := http.NewServeMux()
	health := healthcheck.NewMetricsHandler(r, "elasticsearch-operator")
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))
	mux.HandleFunc("/live", health.LiveEndpoint)
	mux.HandleFunc("/ready", health.ReadyEndpoint)
	
	srv := &http.Server{Handler:mux}
	
	l, err := net.Listen("tcp", ":8000")
	if err != nil {
		logrus.Fatal(err)
	}
	go srv.Serve(l)
	
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			logrus.Error("Shutdown signal received, exiting...")
			close(doneChan)
			wg.Wait()
			os.Exit(0)
		}
	}
}
