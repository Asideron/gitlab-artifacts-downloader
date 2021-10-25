package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Asideron/gitlab-artifacts-downloader/app"
	"github.com/Asideron/gitlab-artifacts-downloader/gitlab"
)

func main() {
	app, err := app.NewApp(context.Background())
	if err != nil {
		fmt.Printf("An error occurred while creating an app instance: %s\n", err.Error())
		os.Exit(-1)
	}

	pipeline := &gitlab.PipelineInfo{
		Project:    app.Config.Project,
		Repository: app.Config.Repository,
		Branch:     app.Config.Branch,
		KeyVals:    app.Config.KeyValues,
	}

	jobsSearch := &gitlab.JobsSearch{
		Jobs: &app.Config.Jobs,
	}

	pipeline.ID, err = app.GitlabCli.TriggerPipeline(pipeline)
	if err != nil {
		fmt.Printf("An error occurred while triggering a pipeline: %s\n", err.Error())
		os.Exit(-1)
	}
	fmt.Println("Pipeline was triggered.")

	jobs, err := app.GitlabCli.FindJobs(app.Ctx, pipeline, jobsSearch)
	if err != nil {
		fmt.Printf("An error occurred while getting jobs: %s\n", err.Error())
		os.Exit(-1)
	}
	fmt.Println("Jobs were located.")

	// TODO: Cancel unneeded jobs.

	artifacts := make(chan *gitlab.Artifact)

	var jobWaitWg sync.WaitGroup
	for _, job := range jobs {
		jobWaitWg.Add(1)
		go func(job *gitlab.JobInfo) {
			defer jobWaitWg.Done()
			ctx, cancel := context.WithTimeout(app.Ctx, app.Config.Timeout)
			defer cancel()
			artifact, err := app.GitlabCli.WaitJobArtifact(ctx, pipeline, job)
			if err != nil {
				fmt.Printf("An error occurred while getting the artifact %s: %s\n", job.Name, err.Error())
				return
			}
			artifacts <- artifact
			fmt.Printf("Got artifact %s. Downloading...\n", artifact.Name)
		}(job)
	}

	go func() {
		jobWaitWg.Wait()
		defer close(artifacts)
	}()

	var downloadWg sync.WaitGroup
	downloadWg.Add(1)
	go func() {
		ticker := time.NewTicker(time.Duration(30) * time.Second)
		defer ticker.Stop()

		defer downloadWg.Done()

		for {
			select {
			case artifact, open := <-artifacts:
				if !open {
					return
				}
				downloadWg.Add(1)
				go func(artifact *gitlab.Artifact) {
					defer downloadWg.Done()
					err := app.GitlabCli.DownloadArtifact(artifact, app.Config.Folder)
					if err != nil {
						fmt.Printf("An error occurred while downloading the artifact %s: %s\n", artifact.Name, err.Error())
						return
					}
					fmt.Printf("Artifact %s was downloaded.\n", artifact.Name)
				}(artifact)
			case <-ticker.C:
				fmt.Println("Waiting...")
			}
		}
	}()
	downloadWg.Wait()

	fmt.Println("Work is finished.")
}
