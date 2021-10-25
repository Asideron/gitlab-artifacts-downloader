package main

import (
	"ci-downloader/gitlab_ci_handler"
	"ci-downloader/models"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
)

type flagsArray []string

func (collection *flagsArray) Set(value string) error {
	*collection = append(*collection, value)
	return nil
}

func (collection *flagsArray) Array() []string {
	return *collection
}

func (collection *flagsArray) String() string {
	return strings.Join(collection.Array(), ", ")
}

func parseFlags() (*models.Config, error) {
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

	return &models.Config{
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
	}, err
}

func main() {
	ctx := context.Background()
	config, err := parseFlags()
	if err != nil {
		fmt.Printf("An error occurred while getting a config: %v\n", err)
		os.Exit(-1)
	}

	client, err := gitlab_ci_handler.NewClient(
		config.Token,
		config.BaseURL,
	)
	if err != nil {
		fmt.Printf("An error occurred in the client creation process: %v\n", err)
		os.Exit(-1)
	}

	println("Searching for artifacts...")
	selectedJobs, err := gitlab_ci_handler.GetJobsWithNeededArtifacts(
		ctx,
		client,
		config,
	)
	if err != nil {
		fmt.Printf(
			"An error occured during the proccess of finding artifacts: %v",
			err,
		)
		os.Exit(-1)
	}
	err = gitlab_ci_handler.DownloadArtifacts(
		client,
		config,
		selectedJobs,
	)
	if err != nil {
		fmt.Printf(
			"An error occured during the proccess of downloading artifacts: %v",
			err,
		)
		os.Exit(-1)
	}
	println("Artifacts were downloaded!")
}
