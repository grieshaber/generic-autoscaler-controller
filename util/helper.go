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

func Min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func Max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func Max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func Abs(a int64) int64 {
	if a < 0 {
		return a * -1
	}
	return a
}
