#!/usr/bin/env -S python3 -u

import json
import urllib.request
import urllib.error

# no API key — proxy is expected to inject it
url = "http://localhost:10000/v1/api-keys"

payload = {
    "name": "test-key",
}

data = json.dumps(payload).encode("utf-8")

req = urllib.request.Request(
    url,
    data=data,
    headers={
        "Content-Type": "application/json",
    },
    method="POST",
)

try:
    with urllib.request.urlopen(req) as resp:
        body = resp.read().decode("utf-8")
        print(json.loads(body))
        print(resp.status)
except urllib.error.HTTPError as e:
    body = e.read().decode("utf-8")
    print(json.loads(body))
    print(e.code)
except urllib.error.URLError as e:
    print({"error": str(e)})
    print("no response status")
