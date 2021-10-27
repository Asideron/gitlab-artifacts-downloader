package gitlab_handler

import (
	"bufio"
	"bytes"
	"ci-downloader/config"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/xanzy/go-gitlab"
)

type jobsInfo map[int]string

func findNeededJobs(
	cli *gitlab.Client,
	conf *config.Config,
	pipelineId int,
) (jobsInfo, error) {
	pipelineJobs, _, err := cli.Jobs.ListPipelineJobs(
		fmt.Sprintf("%s/%s", conf.Project, conf.Repository),
		pipelineId,
		&gitlab.ListJobsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: conf.PagePerCount,
			},
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to fetch jobs from the pipeline %d", pipelineId)
		return nil, err
	}

	var selectedJobs = make(jobsInfo)

	for _, job := range pipelineJobs {
		for _, artifact := range conf.Artifacts {
			if job.Name == artifact {
				selectedJobs[job.ID] = job.Name
			}
		}
	}

	if len(selectedJobs) < len(conf.Artifacts) {
		err = fmt.Errorf("not all needed jobs were found in the pipeline %d", pipelineId)
		return nil, err
	}

	return selectedJobs, err
}

func getJobsFromTriggeredPipeline(
	ctx context.Context,
	cli *gitlab.Client,
	conf *config.Config,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	var pipeline *gitlab.Pipeline
	var variables []*gitlab.PipelineVariable
	var err error
	if conf.Key != "" && conf.Value != "" {
		variables = append(variables, &gitlab.PipelineVariable{
			Key:   conf.Key,
			Value: conf.Value,
		})
	}

	pipeline, _, err = cli.Pipelines.CreatePipeline(
		fmt.Sprintf("%s/%s", conf.Project, conf.Repository),
		&gitlab.CreatePipelineOptions{
			Ref:       gitlab.String(conf.Branch),
			Variables: variables,
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	selectedJobs, err := findNeededJobs(
		cli,
		conf,
		pipeline.ID,
	)
	if err != nil {
		errChan <- err
		return
	}

	for jobID := range selectedJobs {
		for {
			job, _, err := cli.Jobs.GetJob(
				fmt.Sprintf("%s/%s", conf.Project, conf.Repository),
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

			time.Sleep(conf.SleepStep)
		}
	}
	jobsInfoChan <- selectedJobs
}

func getJobsFromFinishedPipeline(
	cli *gitlab.Client,
	conf *config.Config,
	jobsInfoChan chan jobsInfo,
	errChan chan error,
) {
	pipelines, _, err := cli.Pipelines.ListProjectPipelines(
		fmt.Sprintf("%s/%s", conf.Project, conf.Repository),
		&gitlab.ListProjectPipelinesOptions{
			Ref:         gitlab.String(conf.Branch),
			ListOptions: gitlab.ListOptions{PerPage: conf.PagePerCount},
			Status:      gitlab.BuildState("success"),
		},
	)
	if err != nil {
		errChan <- err
		return
	}

	for _, pipeline := range pipelines {
		selectedJobs, err := findNeededJobs(
			cli,
			conf,
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
	cli *gitlab.Client,
	conf *config.Config,
) (jobsInfo, error) {
	jobsInfoChan := make(chan jobsInfo)
	errChan := make(chan error)
	ctx, cancel := context.WithTimeout(ctx, conf.Timeout)
	defer cancel()

	if conf.ForceTrigger {
		println("Triggering new pipeline...")
		go getJobsFromTriggeredPipeline(
			ctx,
			cli,
			conf,
			jobsInfoChan,
			errChan,
		)
	} else {
		println("Searching for the latest suitable pipeline...")
		go getJobsFromFinishedPipeline(
			cli,
			conf,
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
	config *config.Config,
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
