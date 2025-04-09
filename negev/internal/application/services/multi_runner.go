package services

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/carlosrabelo/negev/negev/internal/domain/entities"
)

const DefaultConcurrency = 4

// SwitchRunFunc executes VLAN automation for one switch, writing isolatable
// output to the provided stdout/stderr writers.
type SwitchRunFunc func(sw entities.SwitchConfig, stdout, stderr io.Writer) (entities.RunResult, error)

// MultiRunOptions configures concurrent multi-switch execution.
type MultiRunOptions struct {
	ConfigPath  string
	Write       bool
	Verbose     int
	CreateVLANs bool
	Concurrency int
	AskPass     AskPassFunc
	Stdout      io.Writer
	Stderr      io.Writer
	// Runner overrides per-switch execution (nil → RunTarget).
	Runner SwitchRunFunc
}

// TargetRunResult is the outcome of one switch in a multi-run.
type TargetRunResult struct {
	Label    string
	Target   string
	Platform string
	Result   entities.RunResult
	Err      error
	Stdout   string
	Stderr   string
}

// MultiRunResult aggregates concurrent switch runs.
type MultiRunResult struct {
	Targets []TargetRunResult
}

// ExitCode returns 0 / 1 / 2 per the multi-switch exit rules.
func (r MultiRunResult) ExitCode(write bool) int {
	hadError := false
	hadChanges := false
	for _, t := range r.Targets {
		if t.Err != nil {
			hadError = true
			continue
		}
		if t.Result.HasChanges() {
			hadChanges = true
		}
	}
	if hadError {
		return 1
	}
	if !write && hadChanges {
		return 2
	}
	return 0
}

func (t TargetRunResult) Status(write bool) string {
	if t.Err != nil {
		return "FAILED"
	}
	if !write && t.Result.HasChanges() {
		return "SIMULATED"
	}
	return "OK"
}

func switchLabel(sw entities.SwitchConfig) string {
	if sw.Name != "" {
		return sw.Name
	}
	return sw.Target
}

// RunMulti executes selected switches with a bounded worker pool, buffers
// per-target output from SwitchRunFunc, flushes it with a label prefix, and
// prints a summary table.
func RunMulti(switches []entities.SwitchConfig, opts MultiRunOptions) MultiRunResult {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	if len(switches) == 0 {
		return MultiRunResult{}
	}
	if concurrency > len(switches) {
		concurrency = len(switches)
	}

	runner := opts.Runner
	if runner == nil {
		runner = func(sw entities.SwitchConfig, _, _ io.Writer) (entities.RunResult, error) {
			return RunTarget(sw.Target, opts.ConfigPath, opts.Write, opts.Verbose, opts.CreateVLANs, opts.AskPass)
		}
	}

	type job struct {
		idx int
		sw  entities.SwitchConfig
	}
	jobs := make(chan job)
	results := make([]TargetRunResult, len(switches))
	var flushMu sync.Mutex
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			var outBuf, errBuf bytes.Buffer
			result, err := runner(j.sw, &outBuf, &errBuf)
			tr := TargetRunResult{
				Label:    switchLabel(j.sw),
				Target:   j.sw.Target,
				Platform: j.sw.PlatformID(),
				Result:   result,
				Err:      err,
				Stdout:   outBuf.String(),
				Stderr:   errBuf.String(),
			}
			results[j.idx] = tr

			flushMu.Lock()
			flushPrefixed(stdout, tr.Label, tr.Stdout)
			flushPrefixed(stderr, tr.Label, tr.Stderr)
			if err != nil {
				fmt.Fprintf(stderr, "[%s] ERROR: %v\n", tr.Label, err)
			}
			flushMu.Unlock()
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}
	for i, sw := range switches {
		jobs <- job{idx: i, sw: sw}
	}
	close(jobs)
	wg.Wait()

	printSummary(stdout, results, opts.Write)
	return MultiRunResult{Targets: results}
}

func flushPrefixed(w io.Writer, label, text string) {
	if text == "" {
		return
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for _, line := range lines {
		fmt.Fprintf(w, "[%s] %s\n", label, line)
	}
}

func printSummary(w io.Writer, results []TargetRunResult, write bool) {
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "%-22s %-10s %8s %8s %s\n", "TARGET", "PLATFORM", "CHANGED", "FAILED", "STATUS")
	for _, t := range results {
		fmt.Fprintf(w, "%-22s %-10s %8d %8d %s\n",
			t.Label, t.Platform, t.Result.PortsChanged, t.Result.PortsFailed, t.Status(write))
	}
}

// observeConcurrency wraps a runner to track peak in-flight workers (tests).
func observeConcurrency(runner SwitchRunFunc, inFlight, maxSeen *int32) SwitchRunFunc {
	return func(sw entities.SwitchConfig, stdout, stderr io.Writer) (entities.RunResult, error) {
		cur := atomic.AddInt32(inFlight, 1)
		for {
			prev := atomic.LoadInt32(maxSeen)
			if cur <= prev || atomic.CompareAndSwapInt32(maxSeen, prev, cur) {
				break
			}
		}
		defer atomic.AddInt32(inFlight, -1)
		return runner(sw, stdout, stderr)
	}
}
