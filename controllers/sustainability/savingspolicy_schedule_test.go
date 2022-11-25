/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sustainability

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Schedule", func() {
	Describe("WeekdayRange", func() {
		Context("string", func() {
			wdr, err := NewWeekdayRange("monday", "sunday")
			if err != nil {
				panic(err)
			}

			It("should return full range of days separated by comma", func() {
				Expect(wdr.String()).To(Equal("MON,TUE,WED,THU,FRI,SAT,SUN"))
			})
		})

		Context("iterator", func() {
			It("should iterate over all weekdays", func() {
				wdr, err := NewWeekdayRange("monday", "sunday")
				Expect(err).To(BeNil())

				days := make([]string, 0)
				for wdr.HasNext() {
					days = append(days, wdr.Next())
				}

				Expect(days).To(Equal(weekdays))
			})

			It("should handle iterating cross week boundaries", func() {
				wdr, err := NewWeekdayRange("sunday", "monday")
				Expect(err).To(BeNil())

				days := make([]string, 0)
				for wdr.HasNext() {
					days = append(days, wdr.Next())
				}

				Expect(days).To(Equal([]string{"SUN", "MON"}))
			})
		})
	})

	Describe("WeekdayTime in range", func() {
		Context("UTC", func() {
			now, err := NewWeekdayTimeLocal("Wednesday", "14", "00", "UTC")
			if err != nil {
				panic(err)
			}

			It("should return true when in range", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return true when exact match", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "14", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "14", "00", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return false when after day", func() {
				from, _ := NewWeekdayTimeLocal("Monday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Tuesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return false when before day", func() {
				from, _ := NewWeekdayTimeLocal("Thursday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Sunday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return false when before from", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "14", "01", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return false when after to", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "13", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return true when on from", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "14", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return true when on to", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "14", "00", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})
		})

		Context("Multiple locations", func() {
			now, err := NewWeekdayTimeLocal("Wednesday", "14", "00", "Europe/Stockholm")
			if err != nil {
				panic(err)
			}

			It("should return true when in range", func() {
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return true when exact match", func() {
				h := "13"
				if now.ts.IsDST() {
					h = "12"
				}
				from, _ := NewWeekdayTimeLocal("Wednesday", h, "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", h, "00", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return false when before from", func() {
				fromH := "13"
				if now.ts.IsDST() {
					fromH = "12"
				}
				from, _ := NewWeekdayTimeLocal("Wednesday", fromH, "01", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "15", "00", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return false when after to", func() {
				toH := "12"
				if now.ts.IsDST() {
					toH = "11"
				}
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", toH, "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(false))
			})

			It("should return true when on from", func() {
				fromH := "13"
				if now.ts.IsDST() {
					fromH = "12"
				}
				from, _ := NewWeekdayTimeLocal("Wednesday", fromH, "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", "23", "59", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})

			It("should return true when on to", func() {
				toH := "13"
				if now.ts.IsDST() {
					toH = "12"
				}
				from, _ := NewWeekdayTimeLocal("Wednesday", "00", "00", "UTC")
				to, _ := NewWeekdayTimeLocal("Wednesday", toH, "00", "UTC")
				Expect(now.InRange(from, to)).To(Equal(true))
			})
		})
	})
})
