#!/bin/bash

# Send a POST request to the /corpora endpoint with a JSON payload
curl -X POST http://localhost:8080/corpora \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Sample Corpus",
    "content": "This is a sample corpus content with some words that will be extracted as vocabulary and subphrases."
  }'