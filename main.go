package main

import (
	"generic-autoscaler-controller/pkg/autoscaler"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	autoscaler := autoscaler.New(clientset, 1*time.Second)
	go autoscaler.Run()

}
