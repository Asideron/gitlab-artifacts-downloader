package config

import (
	"flag"
	"fmt"
	"time"
)

const (
	perPageCount = 100
	timeout      = 1800 * time.Second
	sleepStep    = 10 * time.Second
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
	noTrigger := flag.Bool("no-trigger", false,
		"[optional] Do not trigger build if no artifatcts were found")
	forceTrigger := flag.Bool("force-trigger", false,
		"[optional] Force trigger build")
	key := flag.String("k", "", "Name of variable for pipeline")
	value := flag.String("v", "", "Value of variable for pipeline")

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
		NoTrigger:      *noTrigger,
		ForceTrigger:   *forceTrigger,
		Key:            *key,
		Value:          *value,
		Timeout:        timeout,
		SleepStep:      sleepStep,
		PagePerCount:   perPageCount,
	}, err
}
