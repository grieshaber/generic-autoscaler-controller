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

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	v1 "github.com/grieshaber/generic-autoscaler-controller/pkg/apis/autoscalingrule/v1"
	"github.com/jedib0t/go-pretty/table"
)

func LogTable(metricEvaluations map[*v1.AutoscalingRule]*MetricEvaluation) {
	if len(metricEvaluations) == 0 {
		return
	}

	t := table.NewWriter()

	t.AppendHeader(table.Row{"Rule", "Priority", "Higher", "Violation Count", "Desired Replicas"})

	for rule, me := range metricEvaluations {
		t.AppendRow(table.Row{rule.Name, rule.Spec.Priority, me.Higher, me.ViolationCount, me.Replicas})
	}

	overview := t.Render()

	if log.IsLevelEnabled(log.DebugLevel) {
		fmt.Println(overview)
	}
}
