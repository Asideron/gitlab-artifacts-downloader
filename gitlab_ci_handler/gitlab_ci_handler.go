package gitlab_ci_handler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"ci-downloader/models"

	"github.com/xanzy/go-gitlab"
)

func findNeededJobs(
	client *gitlab.Client,
	config *models.Config,
	pipelineId int,
) (jobsInfo, error) {
	pipelineJobs, _, err := client.Jobs.ListPipelineJobs(
		fmt.Sprintf("%s/%s", config.Project, config.Repository),
		pipelineId,
		&gitlab.ListJobsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: perPageCount,
			},
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to fetch jobs from the pipeline %d", pipelineId)
		return nil, err
	}

	var selectedJobs = make(jobsInfo)

	for _, job := range pipelineJobs {
		for _, artifact := range config.Artifacts {
			if job.Name == artifact {
				selectedJobs[job.ID] = job.Name
			}
		}
	}

	if len(selectedJobs) < len(config.Artifacts) {
		err = fmt.Errorf("not all needed jobs were found in the pipeline %d", pipelineId)
		return nil, err
	}

	return selectedJobs, err
}

func getJobsFromTriggeredPipeline(
	ctx context.Context,
	client *gitlab.Client,
	config *models.Config,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	var pipeline *gitlab.Pipeline
	var variables []*gitlab.PipelineVariable
	var err error
	if config.Key != "" && config.Value != "" {
		variables = append(variables, &gitlab.PipelineVariable{
			Key:   config.Key,
			Value: config.Value,
		})
	}

	pipeline, _, err = client.Pipelines.CreatePipeline(
		fmt.Sprintf("%s/%s", config.Project, config.Repository),
		&gitlab.CreatePipelineOptions{
			Ref:       gitlab.String(config.Branch),
			Variables: variables,
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	selectedJobs, err := findNeededJobs(
		client,
		config,
		pipeline.ID,
	)
	if err != nil {
		errChan <- err
		return
	}

	for jobID := range selectedJobs {
		for {
			job, _, err := client.Jobs.GetJob(
				fmt.Sprintf("%s/%s", config.Project, config.Repository),
				jobID,
			)
			if err != nil {
				errChan <- err
				return
			}

			if job.Status == "success" {
				break
			} else if job.Status == "failed" {
				err := fmt.Errorf("job has failed: %v", job.Name)
				errChan <- err
				return
			} else if job.Status == "manual" {
				err := fmt.Errorf("job is manual: %v", job.Name)
				errChan <- err
				return
			}

			time.Sleep(sleepStep)
		}
	}
	jobsInfoChan <- selectedJobs
}

func getJobsFromFinishedPipeline(
	client *gitlab.Client,
	config *models.Config,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	pipelines, _, err := client.Pipelines.ListProjectPipelines(
		fmt.Sprintf("%s/%s", config.Project, config.Repository),
		&gitlab.ListProjectPipelinesOptions{
			Ref:         gitlab.String(config.Branch),
			ListOptions: gitlab.ListOptions{PerPage: perPageCount},
			Status:      gitlab.BuildState("success"),
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	for _, pipeline := range pipelines {
		selectedJobs, err := findNeededJobs(
			client,
			config,
			pipeline.ID,
		)
		if err == nil {
			jobsInfoChan <- selectedJobs
			return
		}
	}

	err = fmt.Errorf("no suitable pipeline was found")
	errChan <- err
}

func GetJobsWithNeededArtifacts(
	ctx context.Context,
	client *gitlab.Client,
	config *models.Config,
) (jobsInfo, error) {
	jobsInfoChan := make(chan jobsInfo)
	errChan := make(chan error)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if config.ForceTrigger {
		println("Triggering new pipeline...")
		go getJobsFromTriggeredPipeline(
			ctx,
			client,
			config,
			jobsInfoChan,
			errChan,
		)
	} else {
		println("Searching for the latest suitable pipeline...")
		go getJobsFromFinishedPipeline(
			client,
			config,
			jobsInfoChan,
			errChan,
		)
	}
	select {
	case selectedJobs := <-jobsInfoChan:
		fmt.Println("Jobs were aquired.")
		return selectedJobs, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout expired")
	}
}

func downloadArtifact(
	artifact *bytes.Reader,
	jobName string,
	downloadFolder string,
	errChan chan error,
) {
	file, err := os.Create(fmt.Sprintf("%s/%s.zip", downloadFolder, jobName))
	if err != nil {
		errChan <- err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	artifact.WriteTo(writer)
	if err != nil {
		errChan <- err
	}
	errChan <- err
}

func DownloadArtifacts(
	client *gitlab.Client,
	config *models.Config,
	selectedJobs jobsInfo,
) error {
	errChan := make(chan error)
	for jobID, jobName := range selectedJobs {
		artifact, _, err := client.Jobs.GetJobArtifacts(
			fmt.Sprintf("%s/%s", config.Project, config.Repository),
			jobID,
			nil,
		)
		if err != nil {
			return err
		}
		go downloadArtifact(artifact, jobName, config.DownloadFolder, errChan)
	}
	for range selectedJobs {
		err := <-errChan
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClient(token string, baseURL string) (*gitlab.Client, error) {
	client, err := gitlab.NewClient(
		token,
		gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return client, err
}
