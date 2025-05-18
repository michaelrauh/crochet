// filepath: /Users/michaelrauh/dev/crochet/pkg/telemetry/redis_metrics.go
package telemetry

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	// Redis stream length gauge
	redisStreamLength = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_stream_messages",
			Help: "Current number of messages in Redis streams.",
		},
		[]string{"stream"},
	)
)

func init() {
	prometheus.MustRegister(redisStreamLength)
}

// StreamMetricsCollector collects metrics from Redis streams
type StreamMetricsCollector struct {
	redisClient *redis.Client
	stream      string
	interval    time.Duration
	stopCh      chan struct{}
	done        chan struct{}
}

// NewStreamMetricsCollector creates a new Redis stream metrics collector
func NewStreamMetricsCollector(redisClient *redis.Client, stream string, interval time.Duration) *StreamMetricsCollector {
	return &StreamMetricsCollector{
		redisClient: redisClient,
		stream:      stream,
		interval:    interval,
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Start begins collecting Redis stream metrics at regular intervals
func (c *StreamMetricsCollector) Start() {
	go func() {
		defer close(c.done)

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		// Collect immediately on start
		c.collectMetrics()

		for {
			select {
			case <-ticker.C:
				c.collectMetrics()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop ends the metrics collection
func (c *StreamMetricsCollector) Stop() {
	close(c.stopCh)
	<-c.done // Wait for collection to stop
}

// collectMetrics gathers and reports Redis stream metrics
func (c *StreamMetricsCollector) collectMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Get stream information
	info, err := c.redisClient.XInfoStream(ctx, c.stream).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Error getting stream info for %s: %v", c.stream, err)
		}
		// If stream doesn't exist yet, set count to 0
		redisStreamLength.WithLabelValues(c.stream).Set(0)
		return
	}

	// Update metric with stream length
	redisStreamLength.WithLabelValues(c.stream).Set(float64(info.Length))
}
