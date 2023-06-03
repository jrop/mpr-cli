package main

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/semaphore"
)

func stringSliceContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// func setLine(line string) {{{
var setLine_lastLineLength int = 0

func setLine(line string) {
	if len(line) < setLine_lastLineLength {
		fmt.Print("\r" + strings.Repeat(" ", setLine_lastLineLength))
	}
	setLine_lastLineLength = len(line)

	fmt.Print("\r" + line)
}

func doParallel(totalIterations int, maxConcurrency int, work func(int)) error {
	ctx := context.TODO()
	sem := semaphore.NewWeighted(int64(maxConcurrency))

	for i := 0; i < totalIterations; i++ {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		go func(i int) {
			defer sem.Release(1)
			work(i)
		}(i)
	}

	return sem.Acquire(ctx, int64(maxConcurrency))
}
