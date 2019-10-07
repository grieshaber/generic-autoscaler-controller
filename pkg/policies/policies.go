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

package policies

import (
	"math"
)

type AutoscalingPolicy struct {
	name                string
	UpScalingFunction   func(replicas int32) float64
	DownScalingFunction func(replicas int32) float64
}

var (
	Mild       = AutoscalingPolicy{"mild", mildUpscalingFunction, mildDownscalingFunction}
	Medium     = AutoscalingPolicy{"medium", mediumUpscalingFunction, mediumDownscalingFunction}
	Strong = AutoscalingPolicy{"strong", strongUpscalingFunction, strongDownscalingFunction}
)

// DOWNSCALING FUNCTION
func DownScalingFunction(replicas int32, limit int64, desired int64) float64 {
	return float64(replicas) * (float64(limit) / float64(desired))
}

// MILD
func mildUpscalingFunction(replicasOld int32) float64 {
	return math.Max(float64(replicasOld+1), float64(replicasOld)*1.15)
}

func mildDownscalingFunction(replicasOld int32) float64 {
	return math.Min(math.Max(0, float64(replicasOld-1)), float64(replicasOld) * 0.85)
}

// MEDIUM
func mediumUpscalingFunction(replicasOld int32) float64 {
	return math.Max(float64(replicasOld+1), float64(replicasOld)*1.3)
}

func mediumDownscalingFunction(replicasOld int32) float64 {
	return math.Min(math.Max(0, float64(replicasOld-1)), float64(replicasOld) * 0.7)
}

// STRONG
func strongUpscalingFunction(replicasOld int32) float64 {
	return math.Max(float64(replicasOld+1), float64(replicasOld)*1.5)
}

func strongDownscalingFunction(replicasOld int32) float64 {
	return math.Min(math.Max(0, float64(replicasOld-1)), float64(replicasOld) * 0.5)
}

