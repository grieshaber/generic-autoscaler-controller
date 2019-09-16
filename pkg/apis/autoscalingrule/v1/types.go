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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AutoscalingRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AutoscalingRuleSpec `json:"spec"`
}

type AutoscalingRuleSpec struct {
	MetricName string     `json:"metricName"`
	Modes      Modes      `json:"modes"`
	Priority   int32      `json:"priority"`
	Thresholds Thresholds `json:"thresholds"`
	AutoMode   AutoMode   `json:"autoMode"`
}

type Modes struct {
	UpscalingMode   string `json:"upscaling"`
	DownscalingMode string `json:"downscaling"`
}

type Thresholds struct {
	UpperThreshold    resource.Quantity `json:"upperThreshold"`
	LowerThreshold    resource.Quantity `json:"lowerThreshold"`
	MaxViolationCount float64           `json:"maxViolationCount"`
}

type AutoMode struct {
	DeltaMetric   string        `json:"deltaMetric"`
	ValueMetric   string        `json:"valueMetric"`
	ThresholdsAdv ThresholdsAdv `json:"thresholdsAdv"`
}

type ThresholdsAdv struct {
	UpperThresholds   UpperThresholds `json:"upperThresholds"`
	LowerThresholds   LowerThresholds `json:"lowerThresholds"`
	MaxViolationCount float64         `json:"maxViolationCount"`
}

type UpperThresholds struct {
	HighUp resource.Quantity `json:"highUp"`
	LowUp  resource.Quantity `json:"lowUp"`
}

type LowerThresholds struct {
	HighDown resource.Quantity `json:"highDown"`
	LowDown  resource.Quantity `json:"lowDown"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AutoscalingRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoscalingRule `json:"items"`
}
