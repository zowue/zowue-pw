#!/bin/bash
# test webhook with samp le push event

curl -X POST http://localhost:8802/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "ref": "refs/heads/main",
    "before": "0000000000000000000000000000000000000000",
    "after": "abc123def456",
    "repository": {
      "full_name": "zowue/zowue-pw",
      "clone_url": "https://github.com/zowue/zowue-pw.git",
      "html_url": "https://github.com/zowue/zowue-pw"
    },
    "pusher": {
      "name": "test",
      "email": "test@example.com"
    },
    "commits": [],
    "head_commit": {
      "id": "abc123def456789",
      "message": "test commit (w)",
      "timestamp": "2024-03-06T10:00:00Z",
      "author": {
        "name": "Test User",
        "email": "test@example.com",
        "username": "testuser"
      },
      "added": [],
      "removed": [],
      "modified": ["test.go"]
    }
  }'

echo ""
echo "webhook test sent"
