package ingest

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var ingestChunksTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "rag_ingest_chunks_total",
		Help: "Total document chunks ingested into the vector store.",
	},
)
