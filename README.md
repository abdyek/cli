# Umono CLI

Command-line interface for the Umono content management system.

## Installation

Install with a single command:

```bash
curl -fsSL https://umono.io/install.sh | sh
```

### Supported Platforms

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |

### Installation Directory

The CLI is installed to `~/.local/bin` by default.

If this directory is not in your PATH, add the following line to your shell configuration file (`.bashrc`, `.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then restart your terminal or run:

```bash
source ~/.bashrc  # or ~/.zshrc
```

## Quick Start

Create a new website:

```bash
umono create my-website
```

## Themes

`umono theme` holds the commands of theme development.

### Create a theme project

```bash
umono theme create new-theme
```

This creates the directory `new-theme` with:

```
new-theme/
  theme.toml
  assets/
    theme.css      # Umono's default look, the starting point of the theme
  .gitignore       # .dev/ and dist/
  .dev/
    contracts/     # the contract of theme.toml, fetched from umono.dev
```

The questions can be answered with flags. A flag that is given is not asked:

| Flag | `theme.toml` key | Required |
| --- | --- | --- |
| `--id` | `id`, `org/id` | Yes |
| `--name` | `name` | Yes |
| `--contract` | `contract`, `org/id@vMAJOR.MINOR` | Yes |
| `--version` | `version`, `0.1.0` by default | No |
| `--description` | `description` | No |
| `--license` | `license`, an SPDX expression | No |
| `--homepage` | `homepage`, an `https://` URL | No |
| `--repository` | `repository`, an `https://` URL | No |
| `--author-name` | `[[authors]] name` | No |
| `--author-url` | `[[authors]] url`, an `https://` URL | No |

With `--no-input`, or when standard input is not a terminal, nothing is asked and a missing required flag is an error:

```bash
umono theme create new-theme --id acme/minimal --name "Minimal" --contract umono/blog@v0.1 --no-input
```

- An existing file or directory with the project's name is never overwritten.
- If the contract cannot be fetched, the project is still created with a warning. `umono theme dev` and `umono theme build` fetch it when it is missing.
- `create` does not write a `LICENSE` file. With `--license`, it reminds you to add the license text to the project root.

## Requirements

- `curl` or `wget` (for installation)
- Linux or macOS

## License

See the [GitHub repository](https://github.com/umono-cms/cli) for details.
