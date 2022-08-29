# gitlab-artifacts-downloader

A simple implemetation of a downloader for Gitlab-CI artifacts.

## Build

To build a binary, the following command should be executed with the `make` utility:

```
make build
```

The binary will be created as `./bin/ci-downloader`.

## Usage and configuration
To download the required artifacts the following configuration should be provided.

### ENV variables

```
GAD_PROJECT=<your-project-name>
GAD_BRANCH=<your-branch-name>
GAD_URL=<your-git-service-url>
GAD_TOKEN=<your-access-token>
GAD_REPO=<your-repo-name>
```

### Command line args

#### Required flags

`-f` - A path to a folder where to download artifacts. Example: `-f=/my/cool/path`  
`-j` - A list of jobs to download aftifacts from. Example: `-j=job1,job2,job3`, `-j=job1`      

#### Optional flags

`-kv` - A list of key:value notes to be provided for a triggered pipeline. Example: `-kv=k1:v1,k2:v2`, `-kv=k1:v1`    
`-t` - A timeout in seconds to wait for the triggered pipeline. **Default: 1800 sec**. Example: `-t=60` 