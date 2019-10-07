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

package autoscalerv2

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	v1 "github.com/grieshaber/generic-autoscaler-controller/pkg/apis/autoscalingrule/v1"
	"github.com/grieshaber/generic-autoscaler-controller/pkg/metrics"
	"github.com/grieshaber/generic-autoscaler-controller/pkg/policies"
	"github.com/grieshaber/generic-autoscaler-controller/util"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"math"
	"sync"
	"time"
)

var (
	waitGroup                         *sync.WaitGroup
	calmdown                          bool
	previousViolationCountIncreasment float64
)

type Autoscalerv2 struct {
	kubeclientset       *kubernetes.Clientset
	interval            time.Duration
	deploymentNamespace string
	deploymentName 	string
	calmdownIntervals   int64
	rules               map[string]*v1.AutoscalingRule
	metricEvaluations   map[*v1.AutoscalingRule]*util.MetricEvaluation
	minReplicas         int32
	maxReplicas         int32
}

func New(kubeclientset *kubernetes.Clientset, interval time.Duration, deploymentNamespace string,deploymentName string, calmdownIntervals int64, rules map[string]*v1.AutoscalingRule, minReplicas int32, maxReplicas int32) *Autoscalerv2 {
	return &Autoscalerv2{kubeclientset: kubeclientset, interval: interval, deploymentNamespace: deploymentNamespace, deploymentName: deploymentName, calmdownIntervals: calmdownIntervals, rules: rules, metricEvaluations: make(map[*v1.AutoscalingRule]*util.MetricEvaluation),
		minReplicas: minReplicas, maxReplicas: maxReplicas}
}

func (as Autoscalerv2) Run() {
	log.Infof("Autoscalerv2 running with interval %v", as.interval)
	waitGroup = &sync.WaitGroup{}
	remainingCalmdownIntervals := as.calmdownIntervals

	ticker := time.NewTicker(as.interval)
	go func() {
		for range ticker.C {
			if calmdown {
				log.Debugf("Calming down after scaling (remaining calmdown intervals %d/%d)", remainingCalmdownIntervals, as.calmdownIntervals)
				remainingCalmdownIntervals--
				if remainingCalmdownIntervals == 0 {
					remainingCalmdownIntervals = as.calmdownIntervals
					calmdown = false
				}
			} else {
				if err := as.evaluateRules(); err != nil {
					log.Error("Error while evaluating rules", err)
				}
			}
		}
	}()
}

func (as Autoscalerv2) calculateNewReplicas(replicasOld int32, countSlope float64, limit int64, desired int64) float64 {
	switch {
	case countSlope > 1:
		return policies.Strong.UpScalingFunction(replicasOld)
	case countSlope > 0.5:
		return policies.Medium.UpScalingFunction(replicasOld)
	case countSlope > 0:
		return policies.Mild.UpScalingFunction(replicasOld)
	case countSlope < 0:
		return policies.DownScalingFunction(replicasOld, limit, desired)
	default:
		return float64(replicasOld)
	}
}

func (as Autoscalerv2) calculateNewViolationCount(rule *v1.AutoscalingRule, value resource.Quantity, delta int64, prevIncreasment float64) float64 {
	metricEvaluation := as.metricEvaluations[rule]
	var (
		newCount float64
	)

	latestCount := metricEvaluation.ViolationCount[len(metricEvaluation.ViolationCount)-1]

	var (
		limit                  int64
		factor                 float64
		violationCountIncrease float64
	)

	valueAsInt := value.MilliValue()
	if delta == 0 {
		violationCountIncrease = prevIncreasment * 0.5
		return latestCount + violationCountIncrease
	} else {
		if delta > 0 {
			// Steigend
			limit = rule.Spec.AutoMode.Limits.UpperLimit.MilliValue()
			factor = 1
		} else if delta < 0 {
			// Fallend
			if valueAsInt < rule.Spec.AutoMode.Limits.LowerLimit.MilliValue() {
				limit = valueAsInt + 2*delta
			} else {
				limit = rule.Spec.AutoMode.Limits.LowerLimit.MilliValue()
			}
			factor = -1
		}

		diffToLimit := util.Abs(valueAsInt - limit)
		intervalsUntilLimit := util.Max64(diffToLimit/util.Abs(delta)-as.calmdownIntervals, 1)
		remainingViolationCount := math.Abs(factor*rule.Spec.AutoMode.Limits.MaxViolationCount - latestCount)
		violationCountIncrease = remainingViolationCount / float64(intervalsUntilLimit)
		newCount = latestCount + factor*violationCountIncrease
		log.Debugf("diff: %v, intsUntil: %v, remainingCount: %v, violationCountIn: %v, newCount: %v", diffToLimit, intervalsUntilLimit, remainingViolationCount, violationCountIncrease, newCount)
	}

	if len(metricEvaluation.ViolationCount) == 5 {
		metricEvaluation.ViolationCount = append(metricEvaluation.ViolationCount[1:], newCount)
	} else {
		metricEvaluation.ViolationCount = append(metricEvaluation.ViolationCount, newCount)
	}

	return violationCountIncrease
}

func (as Autoscalerv2) evaluateRule(rule *v1.AutoscalingRule, replicasOld int32) {
	defer waitGroup.Done()
	log.Debugf("Evaluating rule %s", rule.Name)

	if _, initialized := as.metricEvaluations[rule]; !initialized {
		log.Debugf("Initializing new MetricEvaluation Object for rule %s", rule.Name)
		as.metricEvaluations[rule] = util.NewMetricEvaluation(float64(replicasOld))
	}

	valueMetric, deltaMetric, err := metrics.GetMetrics(as.kubeclientset, rule.Spec.TargetNamespace, rule.Spec.AutoMode)
	if err != nil || len(valueMetric.Items) == 0 {
		log.Errorf("Could not retrieve metrics for rule %s", rule.Name, err)
		return
	}

	value, err := resource.ParseQuantity(valueMetric.Items[0].Value)
	if err != nil {
		log.Errorf("Could not parse value metric for rule %s", rule.Name, err)
		return
	}
	log.Debugf("Current Value: %v", value)

	delta, err := resource.ParseQuantity(deltaMetric.Items[0].Value)
	if err != nil {
		log.Errorf("Could not parse delta metric for rule %s", rule.Name, err)
		return
	}
	log.Debugf("Current Delta: %v", delta)

	metricEvaluation := as.metricEvaluations[rule]

	weightedDelta := int64(0.9*float64(delta.MilliValue()) + 0.1*float64(metricEvaluation.LastDelta))
	metricEvaluation.LastDelta = weightedDelta
	log.Debugf("Current weighted Delta: %v", weightedDelta)

	previousViolationCountIncreasment = as.calculateNewViolationCount(rule, value, weightedDelta, previousViolationCountIncreasment)

	lastViolationCount := metricEvaluation.ViolationCount[len(metricEvaluation.ViolationCount)-1]
	deltaViolationCount := lastViolationCount - metricEvaluation.ViolationCount[0]
	countSlope := deltaViolationCount / float64(len(metricEvaluation.ViolationCount))

	log.Infof("Count slope is %v", countSlope)

	if math.Abs(lastViolationCount) >= rule.Spec.AutoMode.Limits.MaxViolationCount {
		log.Debug("Bingo, need to scale!")
		metricEvaluation.Replicas = as.calculateNewReplicas(replicasOld, countSlope, value.MilliValue(), rule.Spec.AutoMode.Limits.DesiredUsage.MilliValue())

		metricEvaluation.ViolationCount = make([]float64, 1, 5)
	}
}

func (as Autoscalerv2) evaluateRules() error {
	deployments := as.kubeclientset.AppsV1().Deployments(as.deploymentNamespace)
	deployment, err := deployments.Get(as.deploymentName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if deployment.Status.Replicas != deployment.Status.AvailableReplicas {
		// Maybe the old scaling isn't completed yet
		return fmt.Errorf("number of replicas instable, won't scale now")
	}

	log.Debug("Tick. Evaluate all metrics..")
	// asynchronously evaluate metrics
	for _, rule := range as.rules {
		waitGroup.Add(1)
		go as.evaluateRule(rule, deployment.Status.Replicas)
	}
	// Wait for all rules to be evaluated
	waitGroup.Wait()
	log.Debug("All metrics evaluated.")
	util.LogTable(as.metricEvaluations)

	// calculate new replica as a by priority weighted sum
	var (
		weights          int32
		weightedReplicas float64
	)

	for rule, metricEvaluation := range as.metricEvaluations {
		priority := rule.Spec.Priority
		desiredReplicas := metricEvaluation.Replicas

		weights += priority
		weightedReplicas += desiredReplicas * float64(priority)
	}

	newDesiredReplicas := int32(math.Round(weightedReplicas / float64(weights)))

	if newDesiredReplicas != deployment.Status.Replicas {
		log.Infof("New desired replica count: %d", newDesiredReplicas)
		if newDesiredReplicas > as.maxReplicas {
			newDesiredReplicas = as.maxReplicas
		} else if newDesiredReplicas < as.minReplicas {
			newDesiredReplicas = as.minReplicas
		}
		// New desired Replicas! Should scale..
		deployment.Spec.Replicas = &newDesiredReplicas
		_, err := deployments.Update(deployment)

		if err == nil {
			log.Info("Scaled deployment!")
			calmdown = true
		}
		return err
	}

	return err
}
