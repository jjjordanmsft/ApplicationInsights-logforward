package main

import (
	"time"

	"github.com/Microsoft/ApplicationInsights-Go/appinsights"
	"github.com/jjjordanmsft/ApplicationInsights-logforward/common"
)

type statusReader struct {
	activeChan  chan int
	readingChan chan int
	writingChan chan int
	waitingChan chan int
}

func newStatusReader() *statusReader {
	result := &statusReader{
		activeChan:  make(chan int),
		readingChan: make(chan int),
		writingChan: make(chan int),
		waitingChan: make(chan int),
	}

	go result.start()

	return result
}

func (reader *statusReader) start() {
	timer := time.NewTicker(time.Minute)

	var activeSamples []float64
	var readingSamples []float64
	var writingSamples []float64
	var waitingSamples []float64

	for {
		select {
		case active := <-reader.activeChan:
			activeSamples = append(activeSamples, float64(active))
		case reading := <-reader.readingChan:
			readingSamples = append(readingSamples, float64(reading))
		case writing := <-reader.writingChan:
			writingSamples = append(writingSamples, float64(writing))
		case waiting := <-reader.waitingChan:
			waitingSamples = append(waitingSamples, float64(waiting))

		case <-timer.C:
			if len(activeSamples) > 0 {
				metric := appinsights.NewAggregateMetricTelemetry("Nginx Active Connections")
				metric.AddSampledData(activeSamples)
				common.Track(metric)
				activeSamples = activeSamples[:0]
			}

			if len(readingSamples) > 0 {
				metric := appinsights.NewAggregateMetricTelemetry("Nginx Reading Connections")
				metric.AddSampledData(readingSamples)
				common.Track(metric)
				readingSamples = readingSamples[:0]
			}

			if len(writingSamples) > 0 {
				metric := appinsights.NewAggregateMetricTelemetry("Nginx Writing Connections")
				metric.AddSampledData(writingSamples)
				common.Track(metric)
				writingSamples = writingSamples[:0]
			}

			if len(waitingSamples) > 0 {
				metric := appinsights.NewAggregateMetricTelemetry("Nginx Waiting Connections")
				metric.AddSampledData(waitingSamples)
				common.Track(metric)
				waitingSamples = waitingSamples[:0]
			}
		}
	}
}

func (reader *statusReader) SampleActiveConnections(count int) {
	reader.activeChan <- count
}

func (reader *statusReader) SampleReadingConnections(count int) {
	reader.readingChan <- count
}

func (reader *statusReader) SampleWritingConnections(count int) {
	reader.writingChan <- count
}

func (reader *statusReader) SampleWaitingConnections(count int) {
	reader.waitingChan <- count
}
