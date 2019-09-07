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
	"math"
)

type AutoscalingPolicy struct {
	name                string
	upScalingFunction   func(replicas int32) (float64, error)
	downScalingFunction func(replicas int32) (float64, error)
}

var (
	Mild       = AutoscalingPolicy{"mild", mildUpscalingFunction, mildDownscalingFunction}
	Medium     = AutoscalingPolicy{"mild", mediumUpscalingFunction, mediumDownscalingFunction}
	Aggressive = AutoscalingPolicy{"mild", aggressiveUpscalingFunction, aggressiveDownscalingFunction}
)

// MILD
func mildUpscalingFunction(replicasOld int32) (float64, error) {
	return math.Max(float64(replicasOld+1), float64(replicasOld)*1.15), nil
}

func mildDownscalingFunction(replicasOld int32) (float64, error) {
	return math.Min(float64(replicasOld-1), float64(replicasOld) * 0.85), nil
}

// MEDIUM
func mediumUpscalingFunction(replicasOld int32) (float64, error) {
	return math.Max(float64(replicasOld+1), float64(replicasOld) * 1.3), nil
}

func mediumDownscalingFunction(replicasOld int32) (float64, error) {
	return math.Min(float64(replicasOld-1), float64(replicasOld)* 0.7), nil
}

// AGGRESSIVE
func aggressiveUpscalingFunction(replicasOld int32) (float64, error) {
	return math.Max(float64(replicasOld+1), float64(replicasOld) * 1.5), nil
}

func aggressiveDownscalingFunction(replicasOld int32) (float64, error) {
	return math.Min(float64(replicasOld-1), float64(replicasOld) * 0.5), nil
}

