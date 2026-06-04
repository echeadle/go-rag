package llm

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var llmLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "rag_llm_latency_seconds",
		Help:    "Latency of LLM calls by operation.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"op"}, // "chat" or "embed"
)
