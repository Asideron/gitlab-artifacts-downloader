package main

import (
	"ci-downloader/config"
	"ci-downloader/gitlab_handler"
	"context"
	"fmt"
	"os"
)

func main() {
	ctx := context.Background()
	config, err := config.ParseFlags()
	if err != nil {
		fmt.Printf("An error occurred while getting a config: %v\n", err)
		os.Exit(-1)
	}

	client, err := gitlab_handler.NewClient(
		config.Token,
		config.BaseURL,
	)
	if err != nil {
		fmt.Printf("An error occurred in the client creation process: %v\n", err)
		os.Exit(-1)
	}

	println("Searching for artifacts...")
	selectedJobs, err := gitlab_handler.GetJobsWithNeededArtifacts(
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
	err = gitlab_handler.DownloadArtifacts(
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
