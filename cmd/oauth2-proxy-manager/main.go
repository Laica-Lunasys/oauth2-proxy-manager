package main

import (
	"os"

	"github.com/Laica-Lunasys/oauth2-proxy-manager/logger"
	"github.com/Laica-Lunasys/oauth2-proxy-manager/service"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func auth() (*kubernetes.Clientset, error) {
	// Authentication
	var config *rest.Config

	_, err := rest.InClusterConfig()
	if err != nil {
		// Not Cluster
		kubeconfig := os.Getenv("KUBECONFIG")
		if len(kubeconfig) == 0 {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	} else {
		// In Cluster
		conf, err := rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("Failed get Kubernetes config: %v", err)
		}
		config = conf
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, err
}

func main() {
	logger.Init()

	logrus.Printf("[oauth2-proxy-manager] Initializing...")
	clientset, err := auth()
	if err != nil {
		logrus.Fatal(err)
	}

	// Controller
	controller, err := service.NewController(clientset)

	// Observer
	observer, err := service.NewObserver(clientset, controller)
	observer.Run()
}
