/*
 *  Copyright (C) 2019 Heinrich-Heine-Universitaet Duesseldorf, Institute of Computer Science, Department Operating Systems
 *
 *  This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied
 *  warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with this program.  If not, see <http://www.gnu.org/licenses/>
 */

package metrics

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	v1 "github.com/grieshaber/generic-autoscaler-controller/pkg/apis/autoscalingrule/v1"
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
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"describedObject"`
		Timestamp  time.Time `json:"timestamp"`
		MetricName string    `json:"metricName"`
		Value      string    `json:"value"`
	} `json:"items"`
}

func GetMetric(clientset *kubernetes.Clientset, namespace string, metricName string) (Metric, error) {
	var metric Metric
	data, err := clientset.RESTClient().Get().AbsPath("/apis/custom.metrics.k8s.io/v1beta1/namespaces", namespace, "services/*", metricName).DoRaw()
	if err != nil {
		return metric, err
	}

	err = json.Unmarshal(data, &metric)
	return metric, err
}

func GetMetrics(clientset *kubernetes.Clientset, namespace string, autoMode v1.AutoMode) (Metric, Metric, error) {
	var valueMetric Metric
	var deltaMetric Metric

	valueData, err := clientset.RESTClient().Get().AbsPath("/apis/custom.metrics.k8s.io/v1beta1/namespaces", namespace, "services/*", autoMode.ValueMetric).DoRaw()
	if err != nil {
		log.Infof("Error value: %v", err)
		return valueMetric, deltaMetric, err
	}

	err = json.Unmarshal(valueData, &valueMetric)

	if err != nil {
		log.Infof("Error unmarshalling value: %v", err)
		return valueMetric, deltaMetric, err
	}
	log.Infof("Value: %v", valueMetric)
	deltaData, err := clientset.RESTClient().Get().AbsPath("/apis/custom.metrics.k8s.io/v1beta1/namespaces", namespace, "services/*", autoMode.DeltaMetric).DoRaw()
	if err != nil {
		log.Infof("Error delta: %v", err)
		return valueMetric, deltaMetric, err
	}

	err = json.Unmarshal(deltaData, &deltaMetric)

	if err != nil {
		log.Infof("Error delta: %v", err)
	} else {
		log.Infof("Delta: %v", deltaMetric)
	}

	return valueMetric, deltaMetric, err
}
