package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"out_heap_alarm_go/alert"
	"out_heap_alarm_go/config"
	"out_heap_alarm_go/monitor"
)

const serviceName = "HeapAlarm"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// If a command is given (install/start/stop/...), handle it and exit.
	if len(os.Args) > 1 && handleServiceCommand(os.Args[1:]) {
		return
	}

	configPath := "config.toml"
	// Parse --config flag (used when SCM starts the service).
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			break
		}
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// When running as a service, the working directory is C:\Windows\System32.
	// Resolve relative paths (config, log) based on the config file's directory.
	exeDir := filepath.Dir(configPath)
	resolvedLogFile := cfg.LogFile
	if resolvedLogFile != "" && !filepath.IsAbs(resolvedLogFile) {
		resolvedLogFile = filepath.Join(exeDir, resolvedLogFile)
	}

	// Set up logging: console + optional log file.
	setupLogging(resolvedLogFile)

	log.Println("out_heap_alarm_go starting...")
	log.Printf("Config loaded: monitoring process %q, threshold %d MB, check every %ds",
		cfg.ProcessName, cfg.MemoryThresholdMB, cfg.CheckIntervalSec)
	log.Printf("SMTP config: host=%s:%d, from=%s, to=%v, use_tls=%v",
		cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.To, cfg.SMTP.UseTLS)

	if isServiceRun() {
		if err := runService(cfg, resolvedLogFile); err != nil {
			log.Fatalf("Service error: %v", err)
		}
		return
	}

	runConsole(cfg)
}

// setupLogging configures log output to console and optionally to a file.
func setupLogging(logFile string) {
	var writers []io.Writer
	writers = append(writers, os.Stderr)

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file %q: %v", logFile, err)
		}
		defer f.Close()
		writers = append(writers, f)
		log.Printf("Logging to file: %s", logFile)
	}

	log.SetOutput(io.MultiWriter(writers...))
}

// runConsole runs the monitoring loop in the foreground (console mode).
func runConsole(cfg *config.Config) {
	cooldown := alert.NewPerProcessCooldown(cfg.AlertCooldownSec)
	hostname, _ := os.Hostname()
	checkInterval := time.Duration(cfg.CheckIntervalSec) * time.Second

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Monitoring started. PID: %d, Host: %s", os.Getpid(), hostname)

	for {
		select {
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			return
		case <-time.After(checkInterval):
			checkAndAlert(cfg, cooldown, hostname)
		}
	}
}

// checkAndAlert performs one monitoring cycle.
func checkAndAlert(cfg *config.Config, cooldown *alert.PerProcessCooldown, hostname string) {
	procs, err := monitor.FindAllProcesses(cfg.ProcessName)
	if err != nil {
		log.Printf("Warning: %v (will retry next cycle)", err)
		return
	}

	for _, proc := range procs {
		memMB, err := monitor.GetMemoryMB(proc)
		if err != nil {
			log.Printf("Warning: %v (will retry next cycle)", err)
			continue
		}

		pid := proc.Pid
		procName, _ := proc.Name()

		if memMB > float64(cfg.MemoryThresholdMB) {
			log.Printf("ALERT: %s (PID %d) memory %.1f MB exceeds threshold %d MB",
				procName, pid, memMB, cfg.MemoryThresholdMB)

			if !cooldown.TrySend(pid) {
				log.Printf("Alert suppressed for %s (PID %d) (in cooldown period)", procName, pid)
				continue
			}

			log.Printf("Attempting to send alert email via %s:%d...", cfg.SMTP.Host, cfg.SMTP.Port)

			subject := fmt.Sprintf("[ALERT] %s memory high: %.1f MB", procName, memMB)
			body := fmt.Sprintf(
				"Time: %s\nHost: %s\nProcess: %s (PID %d)\nMemory: %.1f MB\nThreshold: %d MB\n",
				time.Now().Format("2006-01-02 15:04:05"),
				hostname,
				procName,
				pid,
				memMB,
				cfg.MemoryThresholdMB,
			)

			if err := alert.SendMail(&cfg.SMTP, subject, body); err != nil {
				log.Printf("Failed to send alert email: %v", err)
			} else {
				log.Printf("Alert email sent to %v", cfg.SMTP.To)
			}
		} else {
			log.Printf("OK: %s (PID %d) memory %.1f MB (threshold: %d MB)",
				procName, pid, memMB, cfg.MemoryThresholdMB)
			cooldown.Reset(pid)
		}
	}
}
