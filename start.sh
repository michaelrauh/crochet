docker compose up -d --build
echo
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
echo
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
echo
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
echo
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
echo
# Open monitoring dashboards
open http://localhost:16686  # Jaeger UI for distributed tracing
open http://localhost:3000   # Grafana for metrics visualization
open http://localhost:4040   # Pyroscope for continuous profiling

