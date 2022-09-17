package main

import (
	"errors"
)

var (
	errFailedToBootstrap     = errors.New("failed to bootstrap to any bootnode")
	errNoTopic = errors.New("given topic does not exist")
)
