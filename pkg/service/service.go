// Package service provides launchd (macOS) and systemd (Linux) background service helpers.
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const LaunchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.tailwake.daemon</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecutablePath}}</string>
        <string>serve</string>
        <string>--config</string>
        <string>{{.ConfigPath}}</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>{{.LogDir}}/tailwake.log</string>

    <key>StandardErrorPath</key>
    <string>{{.LogDir}}/tailwake-error.log</string>
</dict>
</plist>
`

type ServiceConfig struct {
	ExecutablePath string
	ConfigPath     string
	LogDir         string
}

// InstallLaunchdService creates and registers the launchd plist on macOS.
func InstallLaunchdService(configPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate tailwake executable: %w", err)
	}

	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for binary: %w", err)
	}

	if configPath == "" {
		configPath = "config.json"
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		absConfigPath = configPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	logDir := filepath.Join(homeDir, "Library", "Logs", "TailWake")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "com.tailwake.daemon.plist")

	tmpl, err := template.New("plist").Parse(LaunchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse plist template: %w", err)
	}

	file, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}
	defer file.Close()

	svcCfg := ServiceConfig{
		ExecutablePath: execPath,
		ConfigPath:     absConfigPath,
		LogDir:         logDir,
	}

	if err := tmpl.Execute(file, svcCfg); err != nil {
		return fmt.Errorf("failed to render plist template: %w", err)
	}

	// Load plist into launchctl
	cmd := exec.Command("launchctl", "load", "-w", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load failed: %v (output: %s)", err, string(output))
	}

	fmt.Printf("✅ TailWake launchd daemon installed successfully!\n")
	fmt.Printf("   Plist path: %s\n", plistPath)
	fmt.Printf("   Logs directory: %s\n", logDir)
	return nil
}

// UninstallLaunchdService unloads and removes the launchd plist.
func UninstallLaunchdService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.tailwake.daemon.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("No installed launchd daemon found.")
		return nil
	}

	// Unload launchctl service
	cmd := exec.Command("launchctl", "unload", "-w", plistPath)
	_ = cmd.Run()

	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	fmt.Println("✅ TailWake launchd daemon uninstalled successfully.")
	return nil
}
