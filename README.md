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

## Upgrade

Run in the project root to move the site to the latest Umono release:

```bash
umono upgrade
```

It replaces the Umono binary and `umono.json`, which tells the CLI the release's minimum CLI version and Theme API range. `umono.db`, `.env`, `uploads/` and `themes/` are kept.

The CLI does not know which installed theme is active, so before replacing anything it checks every theme under `themes/` against the new release's Theme API range. If a theme is outside the range, or its `built.toml` cannot be read, the upgrade stops and lists them. `--allow-theme-break` upgrades anyway and only lists them; an active theme among them stops working and its pages render with the fallback.

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

### Build a theme

Run in the theme project:

```bash
umono theme build
```

The theme is checked and built into `dist/{org}_{id}_{version}.umono-theme`, for example `dist/acme_minimal_0.1.0.umono-theme`. The file is a zip with:

- `built.toml`: the values of `theme.toml`, with `theme-api-version` and `built-at`
- `contract.toml`: the contract of `theme.toml`, from `.dev/contracts/` (fetched from umono.dev when missing)
- `assets/`: the canonical `theme.css` and the other assets
- `checksums.json`
- `attributes.toml` and `LICENSE`, when the project has them

`theme-api-version` is detected, never written by hand. The build prints it with its reasons:

```
theme-api-version: 1
  ← LICENSE        pkg.license (API 1)
  ← theme_api_min  1
```

- A failed check stops the build and leaves `dist/` untouched.
- Empty directories under `assets/` are skipped.
- Comments do not go into the package. For a comment starting with `/*!`, the build warns you to move the note to `LICENSE`.
- The CLI does not compile CSS. If your CSS needs a compiler, compile it into `assets/theme.css` yourself before the build, and keep its sources outside `assets/`, for example in `src/`: everything under `assets/` goes into the package.

### Install a theme

Run in the root of a Umono project (v0.9 or later), the directory with `umono.json`:

```bash
umono theme install ~/umono-themes/acme_minimal_0.1.0.umono-theme
```

The theme is checked with the same checks Umono runs when it activates a theme, and installed as `themes/{org}/{id}/{version}/` with read-only files. Activate it in the admin panel. `install` works offline: the checks against umono.dev and the database run on activation.

| Flag | Effect |
| --- | --- |
| `--force` | Reinstall an installed version. The installed theme is not touched: the package waits in `themes-pending/{org}/{id}/{version}/` until the reinstall is approved in the admin panel. |
| `--allow-incompatible-api` | Install a theme whose `theme-api-version` is outside the range of this Umono (`theme_api_min` to `theme_api_current` in `umono.json`). It cannot be activated until Umono supports it. |

A theme with a `theme-api-version` newer than the CLI knows is refused even with `--allow-incompatible-api`: the CLI cannot verify it. Upgrade the CLI.

## Requirements

- `curl` or `wget` (for installation)
- Linux or macOS

## License

See the [GitHub repository](https://github.com/umono-cms/cli) for details.
