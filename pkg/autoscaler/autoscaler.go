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

package autoscaler

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	v1 "github.com/grieshaber/generic-autoscaler-controller/pkg/apis/autoscalingrule/v1"
	"github.com/grieshaber/generic-autoscaler-controller/util"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"math"
	"sync"
	"time"
)

var waitGroup *sync.WaitGroup

type Autoscaler struct {
	kubeclientset     *kubernetes.Clientset
	interval          time.Duration
	rules             map[string]v1.AutoscalingRule
	metricEvaluations map[*v1.AutoscalingRule]*util.MetricEvaluation
	shouldScale       bool
}

func New(kubeclientset *kubernetes.Clientset, interval time.Duration, rules map[string]v1.AutoscalingRule) *Autoscaler {
	return &Autoscaler{kubeclientset: kubeclientset, interval: interval, rules: rules, metricEvaluations: make(map[*v1.AutoscalingRule]*util.MetricEvaluation)}
}

func (as Autoscaler) Run() {
	log.Infof("Autoscaler running with interval %v", as.interval)
	waitGroup = &sync.WaitGroup{}

	ticker := time.NewTicker(as.interval)
	go func() {
		for range ticker.C {
			if err := as.evaluateRules(); err != nil {
				log.Error("Error while evaluating rules", err)
			}
		}
	}()
}

func (as Autoscaler) evaluateRule(rule v1.AutoscalingRule, replicasOld int32) {
	defer waitGroup.Done()
	log.Debugf("Evaluating rule %s", rule.Name)
	fmt.Println(as.metricEvaluations[&rule])
	fmt.Println(&rule)
	fmt.Println(len(as.metricEvaluations))
	if _, initialized := as.metricEvaluations[&rule]; !initialized {
		log.Debugf("Initializing new MetricEvaluation Object for rule %s", rule.Name)
		as.metricEvaluations[&rule] = &util.MetricEvaluation{Replicas: float64(replicasOld)}
	}

	metricEvaluation := as.metricEvaluations[&rule]
	metric, err := getMetrics(as.kubeclientset, rule.Spec.MetricName)

	if err != nil {
		log.Panicf("Could not retrieve metrics for rule %s", rule.Name, err)
	}

	value, err := resource.ParseQuantity(metric.Items[0].Value)
	if err != nil {
		log.Panicf("Could not parse metric for rule %s", rule.Name, err)
	}

	if value.Cmp(rule.Spec.Thresholds.UpperThreshold)+1 >= 1 {
		// UpperThreshold reached
		log.Debugf("Upper threshold reached for rule %s", rule.Name)
		if metricEvaluation.Higher {
			metricEvaluation.ViolationCount++
			if metricEvaluation.ViolationCount >= rule.Spec.Thresholds.MaxViolationCount {
				log.Debugf("Max violation count %f reached for rule %s", rule.Spec.Thresholds.MaxViolationCount, rule.Name)
				// calc new replicas!
				var (
					newReplicas float64
					err         error
				)
				switch rule.Spec.Modes.UpscalingMode {
				case "mild":
					newReplicas, err = Mild.upScalingFunction(replicasOld)
				case "medium":
					newReplicas, err = Medium.upScalingFunction(replicasOld)
				case "aggressive":
					newReplicas, err = Aggressive.upScalingFunction(replicasOld)
				default:
					log.Warn("Unsupported scaling mode: %s", rule.Spec.Modes.UpscalingMode)
				}

				if err != nil {
					log.Error("Could not calculate new replicas", err)
					return
				}
				metricEvaluation.Replicas = newReplicas
				as.shouldScale = true
				return
			}
		}
		// Reset violation count, if latest violation was at lower threshold
		metricEvaluation.Higher = true
		metricEvaluation.ViolationCount = 1
		return
	} else if value.Cmp(rule.Spec.Thresholds.LowerThreshold)-1 <= -1 {
		log.Debugf("Lower threshold reached for rule %s", rule.Name)
		// lowerThreshold reached
		if !metricEvaluation.Higher {
			metricEvaluation.ViolationCount++
			if metricEvaluation.ViolationCount >= rule.Spec.Thresholds.MaxViolationCount {
				log.Debugf("Max violation count %f reached for rule %s", rule.Spec.Thresholds.MaxViolationCount, rule.Name)
				// calc new replicas!
				var (
					newReplicas float64
					err         error
				)
				switch rule.Spec.Modes.DownscalingMode {
				case "mild":
					newReplicas, err = Mild.downScalingFunction(replicasOld)
				case "medium":
					newReplicas, err = Medium.downScalingFunction(replicasOld)
				case "aggressive":
					newReplicas, err = Aggressive.downScalingFunction(replicasOld)
				default:
					log.Warn("Unsupported scaling mode: %s", rule.Spec.Modes.UpscalingMode)
				}

				if err != nil {
					log.Error("Could not calculate new replicas", err)
					return
				}
				metricEvaluation.Replicas = newReplicas
				as.shouldScale = true
				return
			}
		}
		// Reset violation count, if latest violation was at upper threshold
		metricEvaluation.Higher = false
		metricEvaluation.ViolationCount = 1
		return
	}
	// if metric is in normal range again, reduce violation count
	log.Debugf("Metric for rule %s in normal range", rule.Name)
	if metricEvaluation.ViolationCount >= 0.33 {
		metricEvaluation.ViolationCount -= 0.33
	}
	return
}

func (as Autoscaler) evaluateRules() error {
	deployments := as.kubeclientset.AppsV1().Deployments("workload-sim")
	deployment, err := deployments.Get("workload-sim-dummy", metav1.GetOptions{})

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

	util.LogTable(as.metricEvaluations)
	log.Debug("All metrics evaluated.")

	if as.shouldScale {
		log.Info("Metrics fired, calculate new replica count..")
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

		log.Info("New desired replica count: %d", newDesiredReplicas)
		if newDesiredReplicas != deployment.Status.Replicas {
			// New desired Replicas! Should scale..
			deployment.Spec.Replicas = &newDesiredReplicas

			// _, err := deployments.Update(deployment)

			return err
		}
	}

	return err
}
