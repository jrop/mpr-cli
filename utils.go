package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"
)

func dbg[T any](t T, msgs ...string) T {
	// get the stack frame of the caller:
	pc, _, _, _ := runtime.Caller(1)
	f := runtime.FuncForPC(pc)

	if len(msgs) == 0 {
		fmt.Printf("%+v (%s)\n", t, f.Name())
	} else {
		fmt.Printf("%+v: %+v (%s)\n", msgs, t, f.Name())
	}

	return t
}

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

func doParallel(totalIterations int, maxConcurrency int, work func(int) error) error {
	ctx := context.TODO()
	sem := semaphore.NewWeighted(int64(maxConcurrency))

	mu := sync.Mutex{}
	errors := make([]error, totalIterations)

	for i := 0; i < totalIterations; i++ {
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		go func(i int) {
			defer sem.Release(1)
			if err := work(i); err != nil {
				mu.Lock()
				errors[i] = err
				mu.Unlock()
			}
		}(i)
	}

	err := sem.Acquire(ctx, int64(maxConcurrency))
	if err != nil {
		return err
	}

	for _, e := range errors {
		if e != nil {
			return e
		}
	}
	return nil
}

type progressReader struct {
	progress    int64
	totalLength int64
	onProgress  func(progress int64, totalLength int64)
}

func (p progressReader) Write(data []byte) (int, error) {
	p.progress += int64(len(data))
	p.onProgress(p.progress, p.totalLength)
	return len(data), nil
}

func newProgressReader(totalLength int64, onProgress func(progress int64, totalLength int64)) progressReader {
	return progressReader{
		progress:    0,
		totalLength: totalLength,
		onProgress:  onProgress,
	}
}
