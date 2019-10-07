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

package main

import (
	log "github.com/Sirupsen/logrus"
	v1 "github.com/grieshaber/generic-autoscaler-controller/pkg/apis/autoscalingrule/v1"
	"github.com/grieshaber/generic-autoscaler-controller/pkg/autoscalerv2"
	"github.com/grieshaber/generic-autoscaler-controller/pkg/client/clientset/versioned"
	"github.com/grieshaber/generic-autoscaler-controller/pkg/client/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"time"
)

var rules = make(map[string]*v1.AutoscalingRule)

func init() {
	log.SetLevel(log.DebugLevel)
}

func getConfig() *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panic("Could not retrieve config", err)
	}
	return config
}

func getKubernetesClientset(config *rest.Config) *kubernetes.Clientset {
	// creates the clientset for scalable application
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic("Could not retrieve clientset for scalable application", err)
	}
	return clientset
}

func getRulesClientset(config *rest.Config) *versioned.Clientset {
	// creates the clientset for autoscaling rules crd
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		log.Panic("Could not retrieve clientset for autoscaling rules", err)
	}
	return clientset
}

func createRulesInformer(rulesClientset *versioned.Clientset) cache.SharedIndexInformer {
	factory := externalversions.NewSharedInformerFactory(rulesClientset, 0)
	informer := factory.Bsinfo().V1().AutoscalingRules().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		DeleteFunc: onDelete,
	})
	return informer
}

func main() {
	log.Info("Start GA-Controller..")
	stopChan := make(chan struct{})
	defer close(stopChan)

	config := getConfig()
	clientset := getKubernetesClientset(config)
	rulesClientset := getRulesClientset(config)

	log.Debug("Create informer to keep track of autoscaling rules..")
	informer := createRulesInformer(rulesClientset)
	go informer.Run(stopChan)
	log.Info("Infomer started.")

	log.Debug("Start autoscaler..")
	scaler := autoscalerv2.New(clientset, 5*time.Second, "workload-sim", "workload-sim-dummy" ,2, rules, 0, 10)
	// scaler := autoscaler.New(clientset, 5*time.Second, 2, rules, 0, 10)
	go scaler.Run()

	<-stopChan
	log.Info("Stopped application!")
}

func onAdd(obj interface{}) {
	rule := obj.(*v1.AutoscalingRule)
	rules[rule.Name] = rule
	log.Infof("Rule added: %s", rule.Name)
}
func onDelete(obj interface{}) {
	rule := obj.(*v1.AutoscalingRule)
	delete(rules, rule.Name)
	log.Infof("Rule %s deleted", rule.Name)
}
