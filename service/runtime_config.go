package service

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

const runtimeConfigPath = "config/path.properties"

type RuntimePathConfig struct {
	AgentFileUploads string
	WebFileUploads   string
	WebFile          string
	WebFileOutput    string
}

var (
	runtimeConfigOnce sync.Once
	runtimeConfig     RuntimePathConfig
)

func GetRuntimePathConfig() RuntimePathConfig {
	runtimeConfigOnce.Do(func() {
		runtimeConfig = RuntimePathConfig{
			AgentFileUploads: "uploads",
			WebFileUploads:   "uploads",
			WebFile:          "File",
			WebFileOutput:    "output",
		}

		file, err := os.Open(runtimeConfigPath)
		if err != nil {
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if value == "" {
				continue
			}

			switch key {
			case "agent.file.uploads":
				runtimeConfig.AgentFileUploads = value
			case "web.File.uploads":
				runtimeConfig.WebFileUploads = value
			case "web.File":
				runtimeConfig.WebFile = value
			case "web.File.output":
				runtimeConfig.WebFileOutput = value
			}
		}
	})

	return runtimeConfig
}
