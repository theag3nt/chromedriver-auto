//go:build linux || darwin
// +build linux darwin

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
)

var (
	versionPattern  = regexp.MustCompile(`(\d+\.?){3,}`)
	browserBinaries = []string{
		"google-chrome",
		"chrome",
		"chromium",
		"chromium-browser",
	}
)

func getInstalledVersion() string {
	log.Print("Searching for browser binaries")
	var path string
	for _, binary := range browserBinaries {
		binaryPath, err := exec.LookPath(binary)
		if err != nil {
			log.Printf("Could not find browser in path: %s", binary)
			continue
		}
		binaryPath, err = filepath.Abs(binaryPath)
		if err != nil {
			log.Printf("Could not construct absolute file path: %v", err)
			continue
		}
		path = binaryPath
	}

	if path == "" {
		log.Printf("No browsers were found in path")
		return ""
	}

	output, err := exec.Command(path, "--version").Output()
	if err != nil {
		log.Printf("Could not run browser (%s): %v", path, err)
		return ""
	}

	version := versionPattern.FindString(string(output))
	if version != "" {
		log.Printf("Found version from %s: %s", filepath.Base(path), version)
	}
	return version
}

func runDriver(path string) {
	cmd := filepath.Base(path)
	args := append([]string{cmd}, os.Args[1:]...)
	env := os.Environ()

	log.Printf("Execing driver process")
	err := syscall.Exec(path, args, env)
	if err != nil {
		log.Fatalf("Error while running driver: %v", err)
	}
}
