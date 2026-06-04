package web

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var requestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rag_requests_total",
		Help: "Total HTTP requests by route pattern and status code.",
	},
	[]string{"route", "status"},
)
