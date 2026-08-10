# pluget

CLI tool to download Minecraft plugins from Modrinth, Hangar, GitHub Releases, Jenkins, and Maven repositories.

## Install

```bash
go install github.com/vanilla-x/pluget/cmd/pluget@latest
```

Or build from source:

```bash
go build -o pluget ./cmd/pluget
```

## Usage

```bash
pluget -config config.yml -out ./plugins
```

| Flag | Required | Description |
|------|----------|-------------|
| `-config` | yes | Path to the YAML config file |
| `-out` | yes | Directory where JAR files are written |

Downloads run concurrently (up to 8 at a time). If a plugin cannot be resolved or downloaded, a warning is printed and the rest continue. The process exits with status `1` if any plugin failed.

Optional: set `GITHUB_TOKEN` (or `GH_TOKEN`) to raise GitHub API rate limits for `github-releases` sources.

## Configuration

See [config.example.yml](config.example.yml).

```yaml
plugins:
  - source: modrinth
    id: 'chunkful-vanish'
    version: '1.2.1'       # exact, *, or Maven version range
    platform: 'paper'      # optional loader filter

  - source: hangar
    id: 'staffprofiles'
    version: '1.0.1'
    platform: 'paper'      # optional; PAPER preferred when omitted

  - source: jenkins
    host: 'https://ci.lucko.me'
    job: 'LuckPerms'
    build: '1658'          # optional; omit to search latest matching build
    artifact: 'LuckPerms-Bukkit-*.jar'

  - source: github-releases
    repository: 'mcmdev/chatchannels'
    version: '1.1'
    artifact: 'chatchannels-*.jar'

  - source: maven
    host: 'https://repo1.maven.org/maven2/'  # optional; defaults to Maven Central
    group: 'com.google.guava'
    artifact: 'guava'
    version: '[32.0,33.0)'
```

### Version and artifact matching

- **Versions** support exact values, `*` (any), and [Maven version ranges](https://maven.apache.org/enforcer/enforcer-rules/versionRanges.html) such as `[1.0,2.0)`.
- **Artifact** names support `*` wildcards (e.g. `LuckPerms-Bukkit-*.jar`).
- Resolvers list versions/builds newest-first and take the first fully matching candidate (“search farther back” via pagination / older builds).

### Sources

| Source | Notes |
|--------|--------|
| `modrinth` | Uses Modrinth API v2; `platform` maps to loaders |
| `hangar` | Uses Hangar API v1; prefers PAPER when `platform` is omitted |
| `jenkins` | Without `build`, walks builds newest→oldest until `artifact` matches |
| `github-releases` | Matches release tags and asset names |
| `maven` | Reads `maven-metadata.xml`; downloads `{artifact}-{version}.jar` |
