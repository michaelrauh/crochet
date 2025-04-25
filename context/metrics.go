package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Define histogram buckets for processing duration
var (
	// Timer buckets - 0.001s to 10s
	timerBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

	// Histogram for tracking ortho processing duration by shape and position
	orthoProcessingDurationByShapePosition = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "crochet_ortho_processing_duration_seconds",
			Help:    "Histogram of ortho processing duration in seconds by shape and position",
			Buckets: timerBuckets,
		},
		[]string{"shape", "position"},
	)
)

// TrackOrthoProcessingTime is a helper function to track the processing time
// for a particular ortho with shape and position information
func TrackOrthoProcessingTime(shape string, position string, processingFunc func() error) error {
	startTime := time.Now()
	err := processingFunc()
	duration := time.Since(startTime).Seconds()

	// Record the processing time with shape and position labels
	orthoProcessingDurationByShapePosition.WithLabelValues(shape, position).Observe(duration)

	return err
}
