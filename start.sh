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

open http://localhost:16686

