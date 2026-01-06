# exporter

輸出如總做題數、作弊總數等 metrics 給 Prometheus。

## Endpoints

- `/`: Health check that returns HTTP 200 "OK".
- `/metrics`: Metrics available for scraping with Prometheus.
