//go:build windows
// +build windows

package service

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"out_heap_alarm_go/alert"
	"out_heap_alarm_go/config"
	"out_heap_alarm_go/monitor"

	"golang.org/x/sys/windows/svc"
)

// RunAsService starts the monitoring loop as a proper Windows service.
// logFile is the absolute path to the log file (resolved by the caller).
func RunAsService(name string, cfg *config.Config, logFile string) error {
	cooldown := alert.NewPerProcessCooldown(cfg.AlertCooldownSec)
	hostname, _ := os.Hostname()

	r := &runner{
		cfg:      cfg,
		cooldown: cooldown,
		hostname: hostname,
		stopped:  make(chan struct{}),
		logFile:  logFile,
	}

	log.Printf("[Service] Starting service %q...", name)
	err := svc.Run(name, r)
	if err != nil {
		log.Printf("[Service] svc.Run returned error: %v", err)
	}
	return err
}

// runner holds the shared state between the service and console runs.
type runner struct {
	cfg      *config.Config
	cooldown *alert.PerProcessCooldown
	hostname string
	stopped  chan struct{}
	logFile  string
}

// Execute is called by the Windows Service Manager. It implements svc.Handler.
func (r *runner) Execute(args []string, changes <-chan svc.ChangeRequest, events chan<- svc.Status) (ssec bool, errno uint32) {
	// Set up file logging so all subsequent log output goes to the file.
	// os.Stderr here goes to the Windows Event Log — we still include it.
	r.setupServiceLogging()

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	log.Printf("[Service] Execute started, args=%v", args)
	events <- svc.Status{State: svc.StartPending}
	events <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Printf("[Service] Running. Monitoring %q, threshold %d MB",
		r.cfg.ProcessName, r.cfg.MemoryThresholdMB)

	go r.runLoop()

	for {
		select {
		case <-r.stopped:
			log.Printf("[Service] Stopping...")
			events <- svc.Status{State: svc.StopPending}
			return false, 0

		case c := <-changes:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				log.Printf("[Service] Received %v", c.Cmd)
				close(r.stopped)
				return false, 0
			default:
				events <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			}
		}
	}
}

// setupServiceLogging configures the standard logger to write to the log file.
// When running under SCM, os.Stderr goes to the Event Log; we also open
// the file directly so logs are persisted.
func (r *runner) setupServiceLogging() {
	if r.logFile == "" {
		return
	}

	f, err := os.OpenFile(r.logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// Fall back to stderr-only if file cannot be opened.
		log.SetOutput(os.Stderr)
		log.Printf("[Service] Warning: cannot open log file %q: %v", r.logFile, err)
		return
	}

	// Write to both the file and stderr (Event Log).
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// runLoop is the main monitoring cycle.
func (r *runner) runLoop() {
	log.Printf("[Service] runLoop started, interval=%ds", r.cfg.CheckIntervalSec)
	ticker := time.NewTicker(time.Duration(r.cfg.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	// Run first check immediately.
	r.check()

	for {
		select {
		case <-r.stopped:
			return
		case <-ticker.C:
			r.check()
		}
	}
}

func (r *runner) check() {
	procs, err := monitor.FindAllProcesses(r.cfg.ProcessName)
	if err != nil {
		log.Printf("[Service] check: %v", err)
		return
	}

	for _, proc := range procs {
		memMB, err := monitor.GetMemoryMB(proc)
		if err != nil {
			log.Printf("[Service] check: %v", err)
			continue
		}

		pid := proc.Pid
		procName, _ := proc.Name()

		if memMB > float64(r.cfg.MemoryThresholdMB) {
			log.Printf("[Service] ALERT: %s (PID %d) %.1f MB > %d MB",
				procName, pid, memMB, r.cfg.MemoryThresholdMB)

			if !r.cooldown.TrySend(pid) {
				log.Printf("[Service] Alert suppressed for %s (PID %d) (cooldown)", procName, pid)
				continue
			}

			subject := fmt.Sprintf("[ALERT] %s memory high: %.1f MB", procName, memMB)
			body := fmt.Sprintf(
				"Time: %s\nHost: %s\nProcess: %s (PID %d)\nMemory: %.1f MB\nThreshold: %d MB\n",
				time.Now().Format("2006-01-02 15:04:05"),
				r.hostname,
				procName,
				pid,
				memMB,
				r.cfg.MemoryThresholdMB,
			)

			if err := alert.SendMail(&r.cfg.SMTP, subject, body); err != nil {
				log.Printf("[Service] SendMail failed: %v", err)
			} else {
				log.Printf("[Service] Email sent to %v", r.cfg.SMTP.To)
			}
		} else {
			log.Printf("[Service] OK: %s (PID %d) %.1f MB", procName, pid, memMB)
			r.cooldown.Reset(pid)
		}
	}
}
