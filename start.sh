docker compose up -d --build

curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'
curl -s -X POST http://localhost:8080/ingest -H "Content-Type: application/json" -d '{"title": "Test Title", "text": "This is a test message"}'

echo "Opening Jaeger UI in your browser..."
open http://localhost:16686

