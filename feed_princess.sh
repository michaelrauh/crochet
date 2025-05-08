#!/bin/bash

# Check if princess.txt exists
if [ ! -f "princess.txt" ]; then
    echo "Error: princess.txt file not found!"
    echo "Please create this file with the content you want to send."
    exit 1
fi

# Read content from princess.txt
PRINCESS_TEXT=$(cat princess.txt)

echo "Sending content from princess.txt to the repository service..."

# Send request with properly escaped JSON to the repository service's /corpora endpoint
curl -X POST http://localhost:8080/corpora \
  -H "Content-Type: application/json" \
  -d "{\"title\": \"Princess Input\", \"text\": $(jq -Rs . < princess.txt)}"

echo -e "\n\nRequest sent successfully."
echo "You can check the processing status via the repository service."