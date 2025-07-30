# xplane ✈️

A smart project assistant for your shell, designed to give you an intelligent summary of your project's status every time you `cd` into it.

> **Note:** This project is currently a work in progress. The core functionality is in place, but features and tests are still being added and refined.

---

## What is xplane?

`xplane` is a command-line tool that hooks into `direnv` to create a context-aware shell environment. On entering a project directory, it runs a series of configurable commands (like `git status`, `tokei`, and `ripsecrets`) and fetches remote information (like open GitHub Pull Requests).

It compares this "dynamic context" to the last known state. If anything has changed, `xplane` uses a Large Language Model (LLM) to generate a brief, human-readable summary of what's new.

The result is a personal project assistant that can tell you things like:
* "A new PR was opened by Jane Doe."
* "Your local branch is 3 commits behind main."
* "You've added 200 lines of Go code and potentially leaked a new secret."

---

## How It Works

1.  **Triggered by `direnv`:** When you `cd` into a directory with a configured `.envrc`, `xplane` is executed.
2.  **Gathers Context:** It runs the commands defined in your configuration to build a "dynamic context" of the project's current state. This includes local git status, code statistics, and remote pull requests.
3.  **Compares State:** The newly gathered context is compared against the last known state, stored in `.xplane/dynamic_context.txt`. If they are identical, the program prints "✅ No new updates." and exits.
4.  **Builds Prompt:** If the context has changed, `xplane` combines the previous and current dynamic contexts with a user-configurable prompt template located at `.xplane/static_context.txt`.
5.  **Generates Summary:** This final prompt is sent to a configured LLM provider (e.g., Gemini), which generates a summary of the changes.
6.  **Updates State:** The new dynamic context is saved, ready for the next comparison.

---

## Installation & Setup

#### 1. Build the Binary
Clone this repository and build the `xplane` executable.

```sh
git clone <your-repo-url>
cd xplane
go build .
```
Then, move the generated `xplane` binary to a directory in your system's `$PATH` (e.g. `/usr/local/bin`).

#### 2. Install Dependencies
`xplane` relies on a few external tools for its default command set. Please ensure the following are installed and available in your `$PATH`:
+ `git`
+ `tokei` (for code statistics)
+ `ripsecrets` (for finding leaked secrets)

#### 3. Configure `direnv`
In your project's `.envrc` file, simply add the `xplane` command. This will execute it every time you enter the directory.

```sh
# .envrc
# ... other direnv setup ...

# Run the xplane assistant
xplane
```

After setting up your `.envrc`, run `direnv allow` to approve it.

---

## Configuration

`xplane` is configured via environment variables, which you should set in your project's `.envrc` file.

| Variable | Description | Default |
| :--- | :--- | :--- |
| **`XPLANE_COMMANDS`** | A comma-separated list of context-gathering commands to run. You can override the defaults or add your own generic commands. | `git_status,git_log,readme,github_prs,tokei,ripsecrets` |
| **`XPLANE_PROVIDER`** | The LLM provider to use for summaries. Currently supports `gemini_cli` and `gemini` (API). | `gemini_cli` |
| **`XPLANE_MODEL`** | The specific model to use with the selected provider. | `gemini-2.5-pro` |
| **`XPLANE_API_KEY`** | The API key required for API-based providers like `gemini`. | (none) |
| **`GITHUB_TOKEN`** | A Personal Access Token with `repo` scope (read only recommended), required for the `github_prs` command. | (none) |
| **`GITLAB_TOKEN`** | A Personal Access Token, required for the `gitlab_mrs` command (when implemented). | (none) |

#### Example `.envrc`

```bash
# .envrc

# Configure the LLM provider
export XPLANE_PROVIDER="gemini_cli"
export XPLANE_MODEL="gemini-1.5-flash"

# Customize the commands to run
export XPLANE_COMMANDS="git_status,github_prs,tokei"

# Provide required tokens
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxx"
```


The first time you run `xplane` in a project, it will automatically create a `.xplane/static_context.txt` file. You can edit this file to customize the persona and instructions for the LLM.

---

## Roadmap & TODO

- [x] Core state-diffing logic
- [x] Modular `LLMProvider` interface
- [x] Modular `GitProvider` interface
- [x] Configurable command runner via `XPLANE_COMMANDS`
- [x] Support for fork-based workflows (`upstream` remote)
- [x] Implemented `gemini_cli` provider
- [x] Implemented `github` provider
- [ ] Add more methods to `GitProvider`
- [ ] Implement `gitlab` and eventually more niche providers like `gitea` or `codeberg`
- [ ] Implement `ollama` provider
- [ ] Implement `claude code` or other API-based LLM providers
- [ ] Add more built-in context commands
- [ ] Implement fancy output formatting

