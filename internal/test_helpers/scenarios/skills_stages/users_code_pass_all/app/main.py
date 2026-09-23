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


SKILLS_DIR = os.path.join(".claude", "skills")


def parse_frontmatter(text):
    """Split a SKILL.md into its frontmatter fields and its body."""
    if not text.startswith("---"):
        return {}, text.strip()

    parts = text.split("---", 2)

    if len(parts) < 3:
        return {}, text.strip()

    fields = {}

    for line in parts[1].strip().split("\n"):
        if ":" in line:
            key, value = line.split(":", 1)
            fields[key.strip()] = value.strip()

    return fields, parts[2].strip()


def discover_skills():
    """Find every skill on disk, sorted by folder name."""
    if not os.path.isdir(SKILLS_DIR):
        return []

    skills = []

    for folder in sorted(os.listdir(SKILLS_DIR)):
        skill_dir = os.path.join(SKILLS_DIR, folder)
        markdown_path = os.path.join(skill_dir, "SKILL.md")

        if not os.path.isfile(markdown_path):
            continue

        with open(markdown_path, "r") as f:
            fields, body = parse_frontmatter(f.read())

        skills.append(
            {
                "name": fields.get("name", folder),
                "description": fields.get("description", ""),
                "dir": skill_dir,
                "body": body,
                "forked": fields.get("context") == "fork",
            }
        )

    return skills


def build_system_prompt(skills):
    """Disclosure level 1: names and descriptions only, never bodies."""
    if not skills:
        return None

    lines = ["You have access to the following skills:", ""]

    for skill in skills:
        lines.append(f"- {skill['name']}: {skill['description']}")

    lines.append("")
    lines.append(
        "To use a skill, read its instructions from "
        ".claude/skills/<name>/SKILL.md using the read tool."
    )

    return "\n".join(lines)


def substitute_arguments(body, arguments):
    """$ARGUMENTS[N] before $ARGUMENTS, since the former contains the latter."""
    for index, value in enumerate(arguments):
        body = body.replace(f"$ARGUMENTS[{index}]", value)

    body = body.replace("$ARGUMENTS", " ".join(arguments))

    # Descending, so $1 never matches the start of $10.
    for index in range(len(arguments) - 1, -1, -1):
        body = body.replace(f"${index}", arguments[index])

    return body


def render_invocation(skill, arguments):
    body = substitute_arguments(skill["body"], arguments)

    # The body names bundled files relative to the skill's own folder, so the
    # model needs to be told where that folder is.
    return (
        f"Skill: {skill['name']} (located at {skill['dir']})\n"
        f"Paths in the instructions below are relative to that folder.\n\n"
        f"{body}"
    )


def expand_invocations(prompt, skills):
    """Expand the run of /name tokens at the start of the prompt.

    The first token that doesn't name a skill ends the run, and that token plus
    everything after it is the argument text for every skill expanded.
    """
    tokens = prompt.split()
    skills_by_name = {skill["name"]: skill for skill in skills}

    invoked = []
    index = 0

    while index < len(tokens) and tokens[index].startswith("/"):
        skill = skills_by_name.get(tokens[index][1:])

        if skill is None:
            break

        invoked.append(skill)
        index += 1

    arguments = tokens[index:]

    return [(skill, render_invocation(skill, arguments)) for skill in invoked]


def run_agent(client, user_prompt):
    skills = discover_skills()

    messages = []

    system_prompt = build_system_prompt(skills)

    if system_prompt:
        messages.append({"role": "system", "content": system_prompt})

    invocations = expand_invocations(user_prompt, skills)

    if not invocations:
        messages.append({"role": "user", "content": user_prompt})

    for skill, invocation in invocations:
        if not skill["forked"]:
            messages.append({"role": "user", "content": invocation})
            continue

        # A forked skill's body is the entire prompt of a conversation that
        # starts empty, so none of the messages built above go with it. Only
        # its answer comes back.
        result = run_loop(client, [{"role": "user", "content": invocation}])

        messages.append(
            {
                "role": "user",
                "content": f"The /{skill['name']} skill ran in a separate context and returned:\n\n{result}",
            }
        )

    return run_loop(client, messages)


def run_loop(client, messages):
    """Drive one conversation until the model answers without calling a tool."""
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
