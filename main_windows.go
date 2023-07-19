package main

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

const (
	fileExt       = ".exe"
	zipNameSuffix = "win32"

	regWorkaroundPrefix       = `Software\`
	regWorkaroundAltPrefix    = `Software\Wow6432Node`
	regChromeUpdaterStableKey = `Software\Google\Update\Clients\{8A69D345-D564-463c-AFF1-A69D9E530F96}`
	regChromeUpdaterAttr      = "pv" // type: REG_SZ
	regChromeBeaconKey        = `Software\Google\Chrome\BLBeacon`
	regChromeBeaconAttr       = "version" // type: REG_SZ
)

func getRegistryValue(root registry.Key, path, attr string) (value string) {
	var rootPath string
	switch root {
	case registry.LOCAL_MACHINE:
		rootPath = "HKLM"
	case registry.CURRENT_USER:
		rootPath = "HKCU"
	default:
		rootPath = "..."
	}
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		log.Printf("Could not find registry key: %s\\%s", rootPath, path)
		// Workaround for potential alternative key path on 64-bit Windows
		if strings.HasPrefix(path, regWorkaroundPrefix) {
			path = fmt.Sprintf("%s\\%s", regWorkaroundAltPrefix, strings.TrimPrefix(path, regWorkaroundPrefix))
			log.Printf("Attempting to use 64-bit subkey: %s\\%s", rootPath, path)
			k, err = registry.OpenKey(root, path, registry.QUERY_VALUE)
			if err != nil {
				log.Printf("Could not find registry key: %s\\%s", rootPath, path)
				return
			}
		} else {
			return
		}
	}
	defer k.Close()

	value, _, err = k.GetStringValue(attr)
	if err != nil {
		log.Printf("Could not find registry value: %s\\%s -> %s", rootPath, path, regChromeUpdaterAttr)
	}
	return
}

// Based on https://bugs.chromium.org/p/chromium/issues/detail?id=158372
func getInstalledVersion() string {
	candidates := []struct {
		t          string
		root       registry.Key
		path, attr string
	}{
		{"local user beacon", registry.CURRENT_USER, regChromeBeaconKey, regChromeBeaconAttr},
		{"local user updater", registry.CURRENT_USER, regChromeUpdaterStableKey, regChromeUpdaterAttr},
		{"machine-wide updater", registry.LOCAL_MACHINE, regChromeUpdaterStableKey, regChromeUpdaterAttr},
	}

	log.Printf("Searching for version information in registry")
	for _, candidate := range candidates {
		version := getRegistryValue(candidate.root, candidate.path, candidate.attr)
		if version != "" {
			log.Printf("Found version from %s: %s", candidate.t, version)
			return version
		}
	}
	return ""
}

func runDriver(path string) {
	// Because Windows doesn't have an alternative to the execve syscall on Unix
	// we wrap the process and forward all signals
	args := os.Args[1:]
	cmd := exec.Command(path, args...)

	// Share file descriptors
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Start process
	log.Printf("Wrapping driver process")
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Could not start wrapped process: %v", err)
	}

	// Start signal handler
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan)

		// Windows doesn't support sending most signals to the wrapped process, so explicitly kill it instead
		sig := <-sigChan
		log.Printf("Signal received (%v) stopping wrapped process", sig)
		err := cmd.Process.Kill()
		if err != nil {
			log.Fatalf("Could not stop wrapped process: %v", err)
		}
	}()

	// Wait for the process to finish
	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("Error while running driver: %v", exitErr)
			os.Exit(exitErr.ExitCode())
		} else {
			log.Fatalf("Unknown error while running driver: %v", err)
		}
	}
}
