package rag

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var retrievalLatency = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "rag_retrieval_latency_seconds",
		Help:    "Latency of vector store retrieval calls.",
		Buckets: prometheus.DefBuckets,
	},
)
