package main

import (
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

type DXRamMetric struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink string `json:"selfLink"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Kind       string `json:"kind"`
			Namespace  string `json:"namespace"`
			Name       string `json:"name"`
			APIVersion string `json:"apiVersion"`
		} `json:"metadata"`
		Timestamp  time.Time `json:"timestamp"`
		MetricName string    `json:"metricName"`
		Value      string    `json:"value"`
	} `json:"items"`
}

func getMetrics(clientset *kubernetes.Clientset, pods *DXRamMetric) error {
	data, err := clientset.RESTClient().Get().AbsPath("apis/metrics.k8s.io/v1beta1/namespace/dxram/services/peer-service/de_hhu_bsinfo_dxram_Storage_FreeMemory").DoRaw()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &pods)
	return err
}

func main() {
	// creates the in-cluster config
	// https://github.com/kubernetes/client-go/tree/master/examples#configuration
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	var pods DXRamMetric
	err = getMetrics(clientset, &pods)
	if err != nil {
		panic(err.Error())
	}
	for _, m := range pods.Items {
		fmt.Println(m.Metadata.Name, m.Metadata.Namespace, m.Timestamp.String())
	}
}
