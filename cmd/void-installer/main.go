package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"void-installer/internal/installer"
	"void-installer/internal/ui"
)

var version = "0.4.1-dev"

func main() { os.Exit(run()) }
func run() int {
	check := flag.Bool("check", false, "Check the Void Live installation environment without modifying disks")
	mirror := flag.String("mirror", installer.DefaultMirror, "HTTPS mirror root; /current is added automatically")
	showVersion := flag.Bool("version", false, "Print the installer version")
	nvidiaVersion := flag.String("check-nvidia", "", "Verify detected GPUs against the specified NVIDIA driver version")
	flag.Parse()
	if *nvidiaVersion != "" {
		if err := installer.CheckNVIDIA(context.Background(), *nvidiaVersion); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Detected NVIDIA GPUs are listed by the selected driver.")
		return 0
	}
	if *showVersion {
		fmt.Println("Void TUI Installer " + version)
		return 0
	}
	if *check {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		defer stop()
		if err := installer.CheckMirrorContext(ctx, *mirror); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Live environment checks passed.")
		return 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := ui.NewWithVersion(ctx, version)
	defer model.Shutdown()
	if err := model.SetMirror(*mirror); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	program := tea.NewProgram(model, tea.WithContext(ctx), tea.WithoutSignalHandler(), tea.WithFilter(ui.ConsoleKeyFilter(os.Getenv("TERM"))))
	signals := make(chan os.Signal, 1)
	finished := make(chan struct{})
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	defer close(finished)
	go func() {
		for {
			select {
			case sig := <-signals:
				if sig == syscall.SIGINT {
					program.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
				} else {
					cancel()
					return
				}
			case <-finished:
				return
			}
		}
	}()
	_, err := program.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Installer stopped:", err)
		fmt.Fprintln(os.Stderr, "Installation diagnostics, if started: /tmp/void-installer-*.log")
		return 1
	}
	return 0
}
