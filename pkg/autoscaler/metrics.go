package autoscaler

import (
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"time"
)

type Metric struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink string `json:"selfLink"`
	} `json:"metadata"`
	Items []struct {
		DescribedObject struct {
			Kind       string `json:"kind"`
			Namespace  string `json:"namespace"`
			Name       string `json:"name"`
		} `json:"describedObject"`
		Timestamp  time.Time `json:"timestamp"`
		MetricName string    `json:"metricName"`
		Value      string    `json:"value"`
	} `json:"items"`
}

func getMetrics(clientset *kubernetes.Clientset, metric *Metric) error {
	data, err := clientset.RESTClient().Get().AbsPath("/apis/custom.metrics.k8s.io/v1beta1/namespaces/dxram/pods/*/de_hhu_bsinfo_dxram_Storage_UsedMemory").DoRaw()
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &metric)
	return err
}

