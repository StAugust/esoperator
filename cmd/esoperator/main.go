package main

import (
	"flag"
	"time"
	
	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	
	clientset "github.com/staugust/esoperator/pkg/client/clientset/versioned"
	informers "github.com/staugust/esoperator/pkg/client/informers/externalversions"
	"github.com/staugust/esoperator/pkg/signals"
	"github.com/staugust/esoperator/pkg/controller"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.Parse()
	
	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	
	esClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}
	
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	esInformerFactory := informers.NewSharedInformerFactory(esClient, time.Second*30)
	
	controller := controller.NewController(kubeClient, esClient,
		kubeInformerFactory.Core().V1().Pods(),
		esInformerFactory.Augusto().V1().EsClusters())
	
	go kubeInformerFactory.Start(stopCh)
	go esInformerFactory.Start(stopCh)
	
	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
