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
	anomalyDelta                      bool
)

type Autoscalerv2 struct {
	kubeclientset     *kubernetes.Clientset
	interval          time.Duration
	target            util.Target
	calmdownIntervals int64
	rules             map[string]*v1.AutoscalingRule
	metricEvaluations map[*v1.AutoscalingRule]*util.MetricEvaluation
	minReplicas       int
	maxReplicas       int
}

func New(kubeclientset *kubernetes.Clientset, interval time.Duration, target util.Target, calmdownIntervals int64, rules map[string]*v1.AutoscalingRule, minReplicas int, maxReplicas int) *Autoscalerv2 {
	return &Autoscalerv2{kubeclientset: kubeclientset, interval: interval, target: target, calmdownIntervals: calmdownIntervals, rules: rules, metricEvaluations: make(map[*v1.AutoscalingRule]*util.MetricEvaluation),
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
				switch as.target.Kind {
				case "Deployment":
					if err := as.evaluateRulesForDeployments(); err != nil {
						log.Error("Error while evaluating rules", err)
					}
				case "StatefulSet":
					if err := as.evaluateRulesForStatefulSets(); err != nil {
						log.Error("Error while evaluating rules", err)
					}
				}
			}
		}
	}()
}

func (as Autoscalerv2) calculateNewReplicas(replicasOld int32, countSlope float64, limit int64, desired int64) float64 {
	switch {
	case countSlope > 1:
		return math.Min(float64(as.maxReplicas), policies.Strong.UpScalingFunction(replicasOld))
	case countSlope > 0.5:
		return math.Min(float64(as.maxReplicas), policies.Medium.UpScalingFunction(replicasOld))
	case countSlope > 0:
		return math.Min(float64(as.maxReplicas), policies.Mild.UpScalingFunction(replicasOld))
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
		newCount = latestCount + violationCountIncrease
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
		violationCountIncrease = factor * (remainingViolationCount / float64(intervalsUntilLimit))
		newCount = latestCount + violationCountIncrease

		if anomalyDelta {
			// Wait one more intervall
			log.Debug("Anomaly! Won't increase Count now")
			newCount = latestCount
		}

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

	valueMetric, deltaMetric, err := metrics.GetMetrics(as.kubeclientset, rule.Spec.TargetNamespace, rule.Spec.AutoMode)
	if err != nil || len(valueMetric.Items) == 0 {
		log.Errorf("Could not retrieve metrics for rule %s: %v", rule.Name, err)
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

	if _, initialized := as.metricEvaluations[rule]; !initialized {
		log.Debugf("Initializing new MetricEvaluation Object for rule %s", rule.Name)
		as.metricEvaluations[rule] = util.NewMetricEvaluation(float64(replicasOld), delta.MilliValue())
	}

	metricEvaluation := as.metricEvaluations[rule]
	metricEvaluation.NumIterations = metricEvaluation.NumIterations + 1

	if util.Abs(delta.MilliValue()) > 10*metricEvaluation.AvgDelta {
		log.Debugf("Delta seems to be anomal %d -> %d", delta, metricEvaluation.AvgDelta)
		if anomalyDelta {
			anomalyDelta = false
		} else {
			anomalyDelta = true
		}
	} else {
		anomalyDelta = false
	}

	metricEvaluation.AvgDelta = metricEvaluation.AvgDelta + util.Abs(delta.MilliValue())/metricEvaluation.NumIterations

	weightedDelta := int64(math.Round(0.9*float64(delta.MilliValue()) + 0.1*float64(metricEvaluation.LastDelta)))
	metricEvaluation.LastDelta = weightedDelta
	log.Debugf("Current weighted Delta: %v", weightedDelta)

	previousViolationCountIncreasment = as.calculateNewViolationCount(rule, value, weightedDelta, previousViolationCountIncreasment)

	lastViolationCount := metricEvaluation.ViolationCount[len(metricEvaluation.ViolationCount)-1]
	deltaViolationCount := lastViolationCount - metricEvaluation.ViolationCount[0]
	countSlope := deltaViolationCount / float64(len(metricEvaluation.ViolationCount))

	log.Infof("Count slope is %v", countSlope)

	if math.Abs(lastViolationCount) >= rule.Spec.AutoMode.Limits.MaxViolationCount {
		log.Debug("Bingo, need to scale!")
		metricEvaluation.Replicas = as.calculateNewReplicas(replicasOld, countSlope, rule.Spec.AutoMode.Limits.LowerLimit.MilliValue(), rule.Spec.AutoMode.Limits.DesiredUsage.MilliValue())

		metricEvaluation.ViolationCount = make([]float64, 1, 5)
	}
}

func (as Autoscalerv2) evaluateRules(replicas int32) int32 {
	log.Debug("Tick. Evaluate all metrics..")
	// asynchronously evaluate metrics
	for _, rule := range as.rules {
		waitGroup.Add(1)
		go as.evaluateRule(rule, replicas)
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

	return int32(math.Round(weightedReplicas / float64(weights)))
}

func (as Autoscalerv2) evaluateRulesForDeployments() error {
	deployments := as.kubeclientset.AppsV1().Deployments(as.target.Namespace)
	deployment, err := deployments.Get(as.target.Name, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if deployment.Status.Replicas != deployment.Status.ReadyReplicas {
		// Maybe the old scaling isn't completed yet
		return fmt.Errorf("number of replicas instable, won't scale now")
	}

	newDesiredReplicas := as.evaluateRules(deployment.Status.Replicas)

	if newDesiredReplicas != deployment.Status.ReadyReplicas {
		log.Infof("New desired replica count: %d", newDesiredReplicas)

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

func (as Autoscalerv2) evaluateRulesForStatefulSets() error {
	statefulsets := as.kubeclientset.AppsV1().StatefulSets(as.target.Namespace)
	statefulset, err := statefulsets.Get(as.target.Name, metav1.GetOptions{})

	if err != nil {
		return err
	}

	if statefulset.Status.Replicas != statefulset.Status.ReadyReplicas {
		// Maybe the old scaling isn't completed yet
		return fmt.Errorf("number of replicas instable, won't scale now")
	}

	newDesiredReplicas := as.evaluateRules(statefulset.Status.Replicas)

	if newDesiredReplicas != statefulset.Status.ReadyReplicas {
		log.Infof("New desired replica count: %d", newDesiredReplicas)

		// New desired Replicas! Should scale..
		statefulset.Spec.Replicas = &newDesiredReplicas
		_, err := statefulsets.Update(statefulset)

		if err == nil {
			log.Info("Scaled statefulset!")
			calmdown = true
		}
		return err
	}

	return err
}
