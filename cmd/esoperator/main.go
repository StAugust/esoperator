package main

import (
	"net/http"
	"net"
	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/heptiolabs/healthcheck"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"github.com/staugust/esoperator/pkg/controller"
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	appVersion = "0.1.0"
	
	printVersion bool
	baseImage    string
	kubeCfgFile  string
	masterHost   string
)

func init() {
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.StringVar(&baseImage, "baseImage", "skydiscovery/elasticsearch:6.2.4", "Base image to use when spinning up the elasticsearch components.")
	flag.StringVar(&kubeCfgFile, "kubecfg-file", "", "Location of kubecfg file for access to kubernetes master service; --kube_master_url overrides the URL part of this; if neither this nor --kube_master_url are provided, defaults to service account tokens")
	flag.StringVar(&masterHost, "masterhost", "http://127.0.0.1:6443", "Full url to k8s api server")
	flag.Parse()
}

func main() {
	if printVersion {
		fmt.Println("elasticsearch-operator", appVersion)
		os.Exit(0)
	}
	logrus.Info("elasticsearch operator starting up!")
	
	// Print params configured
	logrus.Info("Using Variables:")
	logrus.Infof("   baseImage: %s", baseImage)
	
	k8sconfig, err := rest.InClusterConfig()
	if err != nil {
		logrus.Error("Could not find k8s config")
		logrus.Error(err)
		os.Exit(1)
	}
	
	k8sclient, err := kubernetes.NewForConfig(k8sconfig)
	if err != nil {
		logrus.Error("Could not init k8sclient! ", err)
		os.Exit(1)
	}
	
	controller := controller.NewESController(k8sclient)
	
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
	
	srv := &http.Server{Handler: mux}
	
	l, err := net.Listen("tcp", ":8000")
	if err != nil {
		logrus.Fatal(err)
	}
	go srv.Serve(l)
	go controller.Run(doneChan, &wg)
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
