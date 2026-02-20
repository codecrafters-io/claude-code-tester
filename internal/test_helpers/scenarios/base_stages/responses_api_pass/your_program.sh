#!/usr/bin/env -S python3 -u

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

API_KEY = os.environ["OPENROUTER_API_KEY"]
BASE_URL = os.environ["OPENROUTER_BASE_URL"].rstrip("/")
url = f"{BASE_URL}/responses"


def main():
    p = argparse.ArgumentParser()
    p.add_argument("-p", required=True, dest="prompt")
    args = p.parse_args()

    payload = {
        "model": "anthropic/claude-haiku-4.5",
        "input": args.prompt,
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
            data = json.loads(body)
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        print(body, file=sys.stderr)
        sys.exit(1)

    # Responses API: output[].content[].text
    output = data.get("output") or []
    if not output:
        sys.exit(1)
    content = output[0].get("content") or []
    if not content:
        sys.exit(1)
    text = content[0].get("text", "").strip()
    print(text)


if __name__ == "__main__":
    main()
