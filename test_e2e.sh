
set -e

docker-compose up -d --build

response1=$(curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d '{"title": "Test Title", "text": "This is a test message"}')

if ! echo "$response1" | grep -q '"status":"success"'; then
    echo "failed"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

if ! echo "$response1" | grep -q '"version":1'; then
    echo "failed: version mismatch in first call"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

response2=$(curl -s -X POST http://localhost:8080/ingest \
    -H "Content-Type: application/json" \
    -d '{"title": "Test Title", "text": "This is a test message. Hello world"}')

if ! echo "$response2" | grep -q '"status":"success"'; then
    echo "failed"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

if ! echo "$response2" | grep -q '"version":2'; then
    echo "failed: version mismatch in second call"
    docker-compose logs
    docker-compose down --remove-orphans
    exit 1
fi

docker-compose logs

docker-compose down --remove-orphans
echo "success"