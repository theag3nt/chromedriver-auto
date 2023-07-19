package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	urlBase     = "https://chromedriver.storage.googleapis.com"
	urlLatest   = urlBase + "/LATEST_RELEASE_%s"
	urlDownload = urlBase + "/%s/%s"
	zipNameFmt  = "chromedriver_%s.zip"
	zipFileFmt  = "chromedriver%s"

	dirName = "chromedriver-auto.tmp"
	fileFmt = "chromedriver_v%s%s"
)

func parseVersion(version string) (majorVersion, patchVersion string) {
	parts := strings.Split(version, ".")
	patchVersion = strings.Join(parts[:3], ".")
	majorVersion = parts[0]
	return
}

func getLatestDriverForVersion(version string) (string, error) {
	url := fmt.Sprintf(urlLatest, version)
	log.Printf("Requesting URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not get latest driver version for %v: %w", version, err)
	}
	defer resp.Body.Close()
	latest, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read latest driver version: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Latest driver version for %v: Not found", version)
		return "", nil
	}
	log.Printf("Latest driver version for %v: %s", version, latest)
	return string(latest), nil
}

func tryLatestDriverForVersion(majorVersion, patchVersion string) (string, error) {
	candidates := []struct {
		t, version string
	}{
		{"patch version", patchVersion},
		{"major version", majorVersion},
	}
	for _, candidate := range candidates {
		latestVersion, err := getLatestDriverForVersion(candidate.version)
		if err != nil {
			return "", err
		}
		if latestVersion != "" {
			log.Printf("Found latest driver by %s: %v", candidate.t, latestVersion)
			return latestVersion, nil
		}
	}
	return "", nil
}

func downloadDriverVersion(version string, file *os.File) error {
	zipName := fmt.Sprintf(zipNameFmt, zipNameSuffix)
	url := fmt.Sprintf(urlDownload, version, zipName)
	log.Printf("Requesting URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("could not get driver package %v: %w", version, err)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not download driver package: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not get driver package: not found")
	}

	r := bytes.NewReader(content)
	zr, err := zip.NewReader(r, r.Size())
	if err != nil {
		return fmt.Errorf("could not read driver package: %w", err)
	}

	zipFile := fmt.Sprintf(zipFileFmt, fileExt)
	zf, err := zr.Open(zipFile)
	if err != nil {
		return fmt.Errorf("could not open driver package: %w", err)
	}
	defer zf.Close()

	_, err = io.Copy(file, zf)
	if err != nil {
		return fmt.Errorf("could not write driver file: %w", err)
	}

	return nil
}

func getDriverForVersion(version string) (string, error) {
	tempDir := filepath.Join(os.TempDir(), dirName)
	tempFile := filepath.Join(tempDir, fmt.Sprintf(fileFmt, version, fileExt))

	_, err := os.Stat(tempFile)
	if err == nil {
		log.Printf("Found existing driver file: %v", tempFile)
		return tempFile, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("error while checking driver file: %w", err)
	}

	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return "", fmt.Errorf("could not create temp directory: %w", err)
	}

	log.Printf("Downloading driver file")
	f, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	err = downloadDriverVersion(version, f)
	if err != nil {
		return "", fmt.Errorf("could not create driver file: %w", err)
	}

	log.Printf("Driver file downloaded to: %s", tempFile)
	return tempFile, nil
}

func main() {
	log.Println("Looking for installed version")
	version := getInstalledVersion()
	if version == "" {
		log.Fatal("Could not find installed version")
	}

	majorVersion, patchVersion := parseVersion(version)

	log.Println("Looking for matching driver")
	latestVersion, err := tryLatestDriverForVersion(majorVersion, patchVersion)
	if err != nil {
		log.Fatalf("Could not find driver version: %v", err)
	}
	if latestVersion == "" {
		log.Fatal("Could not find latest driver")
	}

	path, err := getDriverForVersion(latestVersion)
	if err != nil {
		log.Fatalf("Could not get driver: %v", err)
	}

	log.Printf("Starting driver file: %v", path)
	runDriver(path)
}
