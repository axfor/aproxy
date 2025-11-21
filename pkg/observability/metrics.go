package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	ActiveConnections prometheus.Gauge
	TotalQueries      prometheus.Counter
	QueryDuration     prometheus.Histogram
	ErrorsTotal       *prometheus.CounterVec
	PGPoolSize        prometheus.Gauge
	BytesIn           prometheus.Counter
	BytesOut          prometheus.Counter
	PreparedStmts     prometheus.Gauge
	TransactionsTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mysql_pg_proxy_active_connections",
			Help: "Number of active MySQL client connections",
		}),
		TotalQueries: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mysql_pg_proxy_total_queries",
			Help: "Total number of queries processed",
		}),
		QueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "mysql_pg_proxy_query_duration_seconds",
			Help:    "Query execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		}),
		ErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "mysql_pg_proxy_errors_total",
			Help: "Total number of errors by type",
		}, []string{"type"}),
		PGPoolSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mysql_pg_proxy_pg_pool_size",
			Help: "PostgreSQL connection pool size",
		}),
		BytesIn: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mysql_pg_proxy_bytes_in_total",
			Help: "Total bytes received from MySQL clients",
		}),
		BytesOut: promauto.NewCounter(prometheus.CounterOpts{
			Name: "mysql_pg_proxy_bytes_out_total",
			Help: "Total bytes sent to MySQL clients",
		}),
		PreparedStmts: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "mysql_pg_proxy_prepared_statements",
			Help: "Number of active prepared statements",
		}),
		TransactionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "mysql_pg_proxy_transactions_total",
			Help: "Total number of transactions by result",
		}, []string{"result"}),
	}
}

func (m *Metrics) IncActiveConnections() {
	m.ActiveConnections.Inc()
}

func (m *Metrics) DecActiveConnections() {
	m.ActiveConnections.Dec()
}

func (m *Metrics) IncTotalQueries() {
	m.TotalQueries.Inc()
}

func (m *Metrics) ObserveQueryDuration(seconds float64) {
	m.QueryDuration.Observe(seconds)
}

func (m *Metrics) IncErrors(errorType string) {
	m.ErrorsTotal.WithLabelValues(errorType).Inc()
}

func (m *Metrics) SetPGPoolSize(size float64) {
	m.PGPoolSize.Set(size)
}

func (m *Metrics) AddBytesIn(bytes float64) {
	m.BytesIn.Add(bytes)
}

func (m *Metrics) AddBytesOut(bytes float64) {
	m.BytesOut.Add(bytes)
}

func (m *Metrics) SetPreparedStmts(count float64) {
	m.PreparedStmts.Set(count)
}

func (m *Metrics) IncTransactions(result string) {
	m.TransactionsTotal.WithLabelValues(result).Inc()
}
