package main

import (
	"strings"
	"testing"
)

func TestBuildPhaseFioFile(t *testing.T) {
	globalSection := "[global]\nioengine=libaio\n"

	tests := []struct {
		name      string
		workload  *FioWorkload
		phase     []FioJob
		wantJobs  []string
		noWaitFor []string // job names that should NOT have wait_for
	}{
		{
			name: "single job phase",
			workload: &FioWorkload{
				GlobalSection: globalSection,
			},
			phase: []FioJob{
				{
					Name:    "job1",
					Section: "[job1]\nfilename=/dev/sda\nrw=read\n",
				},
			},
			wantJobs: []string{"[job1]"},
		},
		{
			name: "removes wait_for to jobs in other phases",
			workload: &FioWorkload{
				GlobalSection: globalSection,
			},
			phase: []FioJob{
				{
					Name:    "job2",
					Section: "[job2]\nfilename=/dev/sda\nrw=write\nwait_for=job1\n",
					WaitFor: "job1",
				},
			},
			wantJobs:  []string{"[job2]"},
			noWaitFor: []string{"job2"}, // wait_for should be removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := buildPhaseFioFile(tt.workload, tt.phase)

			// Check that all expected jobs are present
			for _, jobName := range tt.wantJobs {
				if !strings.Contains(output, jobName) {
					t.Errorf("Output missing job section %s", jobName)
				}
			}

			// Check that global section is present
			if !strings.Contains(output, "[global]") {
				t.Error("Output missing [global] section")
			}
		})
	}
}

func TestFilterJobSection(t *testing.T) {
	tests := []struct {
		name        string
		job         FioJob
		jobsInPhase map[string]bool
		wantMissing []string
	}{
		{
			name: "removes wait_for when job is in different phase",
			job: FioJob{
				Name:    "job3",
				Section: "[job3]\nfilename=/dev/sda\nwait_for=job1\nrw=read\n",
			},
			jobsInPhase: map[string]bool{
				"job3": true,
			},
			wantMissing: []string{"wait_for=job1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := filterJobSection(tt.job, tt.jobsInPhase)

			for _, missing := range tt.wantMissing {
				if strings.Contains(output, missing) {
					t.Errorf("Output contains content that should be removed: %s", missing)
				}
			}
		})
	}
}
