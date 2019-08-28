package autoscaler

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	"strconv"
	"time"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Autoscaler struct {
	kubeclientset *kubernetes.Clientset
	interval      time.Duration
}

func New(kubeclientset *kubernetes.Clientset, interval time.Duration) *Autoscaler {
	return &Autoscaler{kubeclientset: kubeclientset, interval: interval}
}

func (as Autoscaler) Run() {
	ticker := time.NewTicker(as.interval * time.Second)
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			_ := as.evaluateScaling()
		case <-quit:
			ticker.Stop()
		}
	}
}

func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func (as Autoscaler) calculateNewReplicas(deployment *appsv1.Deployment) int32 {
	curReplicas := deployment.Status.Replicas

	return min(curReplicas + 1, 3)
}

func (as Autoscaler) evaluateScaling() error {
	var metric Metric
	err := getMetrics(as.kubeclientset, &metric)

	// error receiving the metrics
	if err != nil {
		panic(err.Error())
	}

	fmt.Println(metric.Items)
	// check metric against threshold (?) and calc new replica count
	if value, err := strconv.ParseFloat(metric.Items[0].Value, 32); err == nil {
		deployments := as.kubeclientset.AppsV1().Deployments("workload-sim")
		deployment, err := deployments.Get("workload-sim-dummy", metav1.GetOptions{})

		if err != nil {
			return err
		}

		if value > 0.7 {
			newReplicas := as.calculateNewReplicas(deployment)
			deployment.Spec.Replicas = &newReplicas
			_, err := deployments.Update(deployment)

			return err
		}
	}
	return err
}
