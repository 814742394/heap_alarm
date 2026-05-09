//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"out_heap_alarm_go/config"
	"out_heap_alarm_go/service"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// exeDir returns the directory containing the current executable.
func exeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// configPath returns the absolute path to config.toml relative to the executable.
func configPath() string {
	return filepath.Join(exeDir(), "config.toml")
}

// resolveLogFile returns an absolute path for a potentially relative logFile path,
// resolved relative to the executable's directory.
func ResolveLogPath(logFile string) string {
	if logFile == "" || filepath.IsAbs(logFile) {
		return logFile
	}
	return filepath.Join(exeDir(), logFile)
}

// handleServiceCommand processes service management commands (install/start/stop/remove/restart/status).
// Returns true if a command was handled, false otherwise.
func handleServiceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	cmd := args[0]
	switch cmd {
	case "install":
		handleInstall()
		return true
	case "start":
		handleStart()
		return true
	case "stop":
		handleStop()
		return true
	case "remove":
		handleRemove()
		return true
	case "restart":
		handleRestart()
		return true
	case "status":
		handleStatus()
		return true
	}
	return false
}

func handleInstall() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	if _, err := m.OpenService(serviceName); err == nil {
		log.Fatalf("Service %q is already installed. Use 'restart' to restart it.", serviceName)
	}

	cfgPath := configPath()
	h, err := m.CreateService(serviceName, exePath,
		mgr.Config{DisplayName: "Heap Alarm", Description: "Process memory monitoring and alert service"},
		"--config", cfgPath)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}
	h.Close()

	log.Printf("Service %q installed.", serviceName)
}

func handleStart() {
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		log.Fatalf("Service %q is not installed: %v", serviceName, err)
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Service %q started.", serviceName)
}

func handleStop() {
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		log.Fatalf("Service %q is not installed: %v", serviceName, err)
	}
	defer s.Close()

	if _, err := s.Control(svc.Stop); err != nil {
		log.Fatalf("Failed to stop service: %v", err)
	}
	log.Printf("Service %q stopped.", serviceName)
}

func handleRemove() {
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		log.Fatalf("Service %q is not installed: %v", serviceName, err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		log.Fatalf("Failed to delete service: %v", err)
	}
	log.Printf("Service %q removed.", serviceName)
}

func handleRestart() {
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		log.Fatalf("Service %q is not installed: %v", serviceName, err)
	}
	defer s.Close()

	if _, err := s.Control(svc.Stop); err != nil {
		log.Fatalf("Failed to stop service: %v", err)
	}
	log.Printf("Service %q stopped, restarting...", serviceName)

	if err := s.Start(); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Service %q started.", serviceName)
}

func handleStatus() {
	cfg, err := config.LoadConfig(configPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	setupLogging(ResolveLogPath(cfg.LogFile))

	m, err := mgr.Connect()
	if err != nil {
		log.Fatalf("Could not connect to service manager: %v", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		log.Printf("Service %q: not installed", serviceName)
		return
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		log.Fatalf("Failed to query service status: %v", err)
	}
	state := "Unknown"
	switch status.State {
	case 1:
		state = "Stopped"
	case 2:
		state = "Start Pending"
	case 3:
		state = "Stop Pending"
	case 4:
		state = "Running"
	case 5:
		state = "Continue Pending"
	case 6:
		state = "Pause Pending"
	case 7:
		state = "Paused"
	default:
		state = fmt.Sprintf("State=%d", status.State)
	}
	log.Printf("Service %q: %s", serviceName, state)
}

// isServiceRun returns true if the process was launched by the Windows Service Controller.
func isServiceRun() bool {
	isSvc, _ := svc.IsWindowsService()
	return isSvc
}

// runService starts the program as a proper Windows service.
func runService(cfg *config.Config, logFile string) error {
	return service.RunAsService(serviceName, cfg, logFile)
}
