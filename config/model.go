package config

import (
	"strings"
	"time"
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

type Config struct {
	Project        string
	Branch         string
	BaseURL        string
	Token          string
	Repository     string
	DownloadFolder string
	Artifacts      []string
	NoTrigger      bool
	ForceTrigger   bool
	Key            string
	Value          string
	Timeout        time.Duration
	SleepStep      time.Duration
	PagePerCount   int
}
