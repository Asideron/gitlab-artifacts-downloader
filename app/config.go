package app

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
)

const (
	defaultTimeoutSeconds = 1800 // 3 minutes
)

type Config struct {
	Project    string `env:"GAD_PROJECT,notEmpty"`
	Branch     string `env:"GAD_BRANCH,notEmpty"`
	BaseURL    string `env:"GAD_URL,notEmpty"`
	Token      string `env:"GAD_TOKEN,notEmpty"`
	Repository string `env:"GAD_REPO,notEmpty"`

	Jobs      []string
	Folder    string
	KeyValues map[string]string
	Timeout   time.Duration
}

func parseFlags() (*Config, error) {
	usage := func() {
		fmt.Println("Dowloader of gitlab-ci artifacts.")
		flag.PrintDefaults()
	}
	flag.Usage = usage

	cfg := Config{}

	if err := env.Parse(&cfg); err != nil {
		usage()
		return nil, err
	}

	jobs := flag.String("j", "", "List of jobs to extract artifacts from.")
	folder := flag.String("f", ".", "Folder to download artifacts in.")

	// Optional params
	keyValues := flag.String("kv", "", "[optional] Key:value list for a triggered pipeline")
	timeout := flag.Duration("t", defaultTimeoutSeconds, "[optional] Timeout seconds for artifacts download process.")

	flag.Parse()

	if *jobs == "" || *folder == "" {
		usage()
		return nil, errNotAllRequiredFlagsSet
	}

	jobsList := strings.Split(*jobs, ",")
	keyValuesMap := make(map[string]string)
	if keyValuesList := strings.Split(*keyValues, ","); len(keyValuesList) > 1 {
		for _, keyValue := range keyValuesList {
			kv := strings.Split(keyValue, ":")
			keyValuesMap[kv[0]] = kv[1]
		}
	}

	cfg.Jobs = jobsList
	cfg.Folder = *folder
	cfg.KeyValues = keyValuesMap
	cfg.Timeout = *timeout * time.Second

	return &cfg, nil
}
