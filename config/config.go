package config

import (
	"flag"
	"fmt"
	"time"
)

const (
	perPageCount          = 50
	defaultPipelinesLimit = 100
	timeout               = 1800 * time.Second
	sleepStep             = 10 * time.Second
)

func ParseFlags() (*Config, error) {
	usage := func() {
		println("Auto-dowloader of artifacts.")
		flag.PrintDefaults()
	}

	project := flag.String("p", "", "Project name")
	repository := flag.String("r", "", "Repository name")
	branch := flag.String("b", "", "Name of branch")
	token := flag.String("t", "", "Gitlab account token")
	url := flag.String("u", "", "Base URL to the gitlab workspace")

	var artifacts flagsArray

	flag.Var(&artifacts, "a", "List of artifacts")
	downloadFolder := flag.String("d", "", "Download folder")
	forceTrigger := flag.Bool("force-trigger", false,
		"[optional] Force trigger build")
	key := flag.String("k", "", "Name of variable for pipeline")
	value := flag.String("v", "", "Value of variable for pipeline")
	pipelinesLimit := flag.Int("pipelines-limit", defaultPipelinesLimit,
		"[optional] Limit for the pipelines count to be examined during the search")

	flag.Usage = usage
	flag.Parse()

	var err error
	if *project == "" ||
		*repository == "" ||
		*branch == "" ||
		*token == "" ||
		artifacts == nil ||
		*downloadFolder == "" ||
		*url == "" {
		usage()
		err = fmt.Errorf("not all required flags were specified")
	}

	return &Config{
		Project:        *project,
		Branch:         *branch,
		BaseURL:        *url,
		Token:          *token,
		Repository:     *repository,
		DownloadFolder: *downloadFolder,
		Artifacts:      artifacts,
		ForceTrigger:   *forceTrigger,
		Key:            *key,
		Value:          *value,
		Timeout:        timeout,
		SleepStep:      sleepStep,
		PerPageCount:   perPageCount,
		PipelinesLimit: *pipelinesLimit,
	}, err
}
