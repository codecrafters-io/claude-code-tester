import argparse
import json
import os
import subprocess

from openai import OpenAI


def read_file(path):
    try:
        with open(path, "r") as f:
            return f.read()
    except Exception as e:
        return f"Error reading file: {str(e)}"


def write_file(path, content):
    try:
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
            return "OK"
    except Exception as e:
        return f"Error writing to file: {str(e)}"


def bash_command(command):
    try:
        proc = subprocess.run(command, shell=True, capture_output=True, text=True)
        return json.dumps(
            {
                "returncode": proc.returncode,
                "stdout": proc.stdout,
                "stderr": proc.stderr,
            }
        )
    except Exception as e:
        return json.dumps(
            {
                "returncode": -1,
                "stdout": "",
                "stderr": str(e),
            }
        )


TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "read",
            "description": "Read the contents of a file",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "The path of the file to read",
                    }
                },
                "required": ["path"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "write",
            "description": "Write content to a file",
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "The path of the file to write to",
                    },
                    "content": {
                        "type": "string",
                        "description": "The content to write to the file",
                    },
                },
                "required": ["path", "content"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "bash_command",
            "description": "Execute a shell command and return stdout, stderr, and return code",
            "parameters": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "The bash command to execute",
                    },
                },
                "required": ["command"],
            },
        },
    },
]


def run_agent(client, user_prompt):
    messages = [{"role": "user", "content": user_prompt}]

    while True:
        resp = client.chat.completions.create(
            model="anthropic/claude-haiku-4.5",
            messages=messages,
            tools=TOOLS,
        )

        if not resp.choices:
            raise RuntimeError("no choices in response")

        msg = resp.choices[0].message

        # If no tool calls → final answer
        if not msg.tool_calls:
            return msg.content

        # Append assistant message that triggered tool calls
        messages.append(msg)

        # Execute *all* tool calls
        for tool_call in msg.tool_calls:
            name = tool_call.function.name
            args = json.loads(tool_call.function.arguments)

            if name == "read":
                result = read_file(args["path"])
            elif name == "write":
                result = write_file(args["path"], args["content"])
            elif name == "bash_command":
                result = bash_command(args["command"])
            else:
                result = f"Unknown tool: {name}"

            # Feed tool result back to model
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": result,
                }
            )


def main():
    p = argparse.ArgumentParser()
    p.add_argument("-p", required=True)
    args = p.parse_args()

    api_key = os.getenv("OPENROUTER_API_KEY")
    base_url = os.getenv("OPENROUTER_BASE_URL")

    if not api_key:
        raise RuntimeError("OPENROUTER_API_KEY is not set")
    if not base_url:
        raise RuntimeError("OPENROUTER_BASE_URL is not set")

    client = OpenAI(api_key=api_key, base_url=base_url)

    output = run_agent(client, args.p)
    print(output)


if __name__ == "__main__":
    main()
