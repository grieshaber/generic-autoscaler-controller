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

package util

import "k8s.io/apimachinery/pkg/api/resource"

type MetricEvaluation struct {
	ValueHistory   []*resource.Quantity
	ViolationCount []float64
	Replicas       float64
	Higher         bool
}

func NewMetricEvaluation(replicas float64) *MetricEvaluation {
	return &MetricEvaluation{[]*resource.Quantity{resource.NewMilliQuantity(0, resource.DecimalSI), resource.NewMilliQuantity(0, resource.DecimalSI)},
		make([]float64, 1, 5), replicas, false}
}
