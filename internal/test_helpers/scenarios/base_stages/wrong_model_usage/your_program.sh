#!/usr/bin/env -S python3 -u

import json
import urllib.request

API_KEY = "dummy-api-key"
url = "http://localhost:10000/api/v1/chat/completions"

payload = {
    "model": "openai/gpt-5.2-pro",
    "messages": [
        {"role": "user", "content": "Hello"}
    ],
}

data = json.dumps(payload).encode("utf-8")

req = urllib.request.Request(
    url,
    data=data,
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    },
    method="POST",
)

try:
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode("utf-8")
        print(json.loads(body))
except urllib.error.HTTPError as e:
    body = e.read().decode("utf-8")
    print(json.loads(body))
