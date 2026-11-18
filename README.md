<div align="center">
  <img src="https://github.com/DIE-Bonn/MatomoTracking/raw/main/.assets/matomo_tracking_logo.png"
  alt="Matomo_Tracking_Logo" title="Matomo_Tracking_Logo" />
</div>
<p align="center"><em><b>"If blockers block, I still track."</b></em></p>

---

<h1 align="center">
<img alt="GitHub" src="https://img.shields.io/github/license/DIE-Bonn/MatomoTracking?color=blue">
<img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/DIE-Bonn/MatomoTracking?include_prereleases">
<img alt="GitHub go.mod Go version" src="https://img.shields.io/github/go-mod/go-version/DIE-Bonn/MatomoTracking">
<img alt="GitHub issues" src="https://img.shields.io/github/issues/DIE-Bonn/MatomoTracking">
<img alt="GitHub last commit (branch)" src="https://img.shields.io/github/last-commit/DIE-Bonn/MatomoTracking/main">
</h1>

# Matomo Tracking

## Overview

This plugin, `MatomoTracking`, is designed for Traefik as middleware to handle the tracking of requests using the Matomo analytics platform. The plugin inspects incoming HTTP requests, checks if tracking is enabled for the requested domain, and sends tracking data to Matomo if required.

The main purpose of this plugin is to enhance the accuracy of visitor tracking by overcoming limitations associated with the traditional JavaScript-based tracking method used by Matomo. Standard tracking relies on JavaScript code running in the user's browser, which can be blocked by certain browser extensions or privacy tools. By capturing tracking data directly on the server side, this plugin ensures that visitor information is accurately recorded even when JavaScript is disabled or blocked, providing a more reliable and comprehensive analytics solution.

![Matomo Tracking diagram](https://github.com/DIE-Bonn/MatomoTracking/raw/main/.assets/matomo_tracking_network_flow.png)

## Structs and Configuration Explanation

1. `Config` Struct

    **Purpose**:

    The `Config` struct represents the configuration settings for the entire Matomo Tracking plugin. It defines the global settings that apply to all domains and the specific configurations for each domain that needs tracking.

    **Fields:**

    - `MatomoURL`
        - Type: **string**
        - Description: Specifies the base URL for the Matomo server endpoint where tracking data should be sent. Typically, this is the URL to the `matomo.php` file on the Matomo server, such as `https://matomo.example.com/matomo.php`.
        - Example: `"https://matomo.example.com/matomo.php"`
    - `Domains`:
        - Type: `map[string]DomainConfig`
        - Description: A map where each key is a domain name (as a `string`) and the corresponding value is a `DomainConfig` struct. This allows you to define tracking rules for multiple domains individually.
        - Example:
            ```yaml
            domains:
              "www3.example.com":
                trackingEnabled: true
                idSite: 21
                excludedPaths:
                  - "/admin/*"
                  - "\\.\\w{1,5}(\\?.+)?$"
                includedPaths:
                  - "\\.(php|aspx)(\\?.*)?$"
                pathOverrides:
                  "/subdir":
                    trackingEnabled: true
                    idSite: 24
                    excludedPaths:
                      - "/test"
                      - "/test2"
                  "/subdir2":
                    trackingEnabled: false
                    idSite: 24
              "test.de":
                trackingEnabled: false
                idSite: 456
            ```
    - `Debug`:
        - Type: **bool**
        - Description: Logs every tracking decision (tracked, skipped and why) to the Traefik log. Errors are always logged.
        - Example: `true`
    - Top-level tracking rules (`TrackingEnabled`, `IdSite`, `TrackedMethods`, `ExcludedPaths`, `IncludedPaths`, `PathOverrides`, `ResponseConditions`):
        - Description: The same fields as in `DomainConfig`, set directly on the plugin. They apply to every host **without** an entry in `domains`; an entry in `domains` always wins, even if it disables tracking. Without top-level `trackingEnabled: true`, unlisted hosts are not tracked (as before).
        This is mainly meant for middlewares attached to single routers, e.g. defined through Docker labels, where the `domains` map cannot be used. See [docs/docker-labels.md](docs/docker-labels.md).
        - Example:
            ```yaml
            matomoURL: "https://matomo.example.com/matomo.php"
            trackingEnabled: true
            idSite: 3
            trackedMethods:
              - "GET"
            ```

2. `DomainConfig` Struct

    **Purpose**:

    The `DomainConfig` struct defines the tracking settings for a specific domain. Each domain that should be tracked (or explicitly not tracked) has a corresponding `DomainConfig` entry.

    **Fields**:

    - `TrackingEnabled`:
        - **Type**: `bool`
        - **Description**: Indicates whether tracking is enabled for the given domain. If `true`, the plugin will attempt to send tracking data to Matomo. If `false`, tracking is disabled for that domain.
        - **Example**: true
    - `IdSite`:
        - **Type**: `int`
        - **Description**: The unique identifier for the website or domain in Matomo. Each domain has its own site ID that Matomo uses to differentiate between multiple websites being tracked on the same server.
        - **Example**: `21`
    - `TrackedMethods`:
        - **Type**: `[]string` (Slice of strings)
        - **Description**: Optional HTTP method allowlist for tracking. If `trackedMethods` is omitted, all methods are tracked as before. If provided, only listed methods are tracked; matching is case-insensitive. An explicitly empty list (`trackedMethods: []`) tracks no methods. All requests still reach the backend unchanged, regardless of whether they are tracked.
        - **Example**:
            ```yaml
            trackedMethods:
              - "GET"
              - "POST"
            ```
    - `ExcludedPaths`:
        - **Type**: `[]string` (Slice of strings)
        - **Description**: A list of regular expressions that define URL paths that should be excluded from tracking. If the requested path matches any of the regex patterns in this list, the request will not be tracked by Matomo.
        - **Example**:
            ```yaml
            excludedPaths:
              - "/admin/*"
              - "\\.\\w{1,5}(\\?.+)?$"
            ```
    - `IncludedPaths`:
        - **Type**: `[]string` (Slice of strings)
        - **Description**: A list of regular expressions that define URL paths that should be explicitly included for tracking. If a requested path matches any of the regex patterns in this list, the request will be tracked by Matomo, even if it matches an exclusion pattern.
        - **Example**:
            ```yaml
            includedPaths:
              - "\\.(php|aspx)(\\?.*)?$"
            ```
    - `PathOverrides`:
        - **Type**: `map[string]PathConfig`
        - **Description**: A map of path-specific configuration overrides that apply only to requests matching those paths. Each key is a path prefix (e.g., `/api`, `/special`) and its corresponding value is a `PathConfig` block. This feature allows more granular control over tracking behavior within a domain.
        Path overrides support the same fields as the domain-level configuration: `trackingEnabled`, `idSite`, `trackedMethods`, `excludedPaths`, `includedPaths`, and `responseConditions`. Overrides are always merged onto the domain configuration, not onto each other. A path override cannot enable tracking for a domain with `trackingEnabled: false`. If a path override is defined, it will **override** the corresponding settings from the parent domain **only for requests matching that path**. A path without its own `trackedMethods` inherits the domain's method list. A path with its own list replaces that list completely, including when it is explicitly empty.
        Matching is done using **prefix matching with boundary awareness**. This means:
          - `/test` matches `/test` and `/test/something`
          - `/test` does not match `/test2` or `/testing`

### Configuration Breakdown

Let's analyze how the configuration is used in the `dynamic.yml` file:

**Example Configuration**

```yaml
matomo-tracking:
  plugin:
    matomoTracking:
      matomoURL: "https://matomo.example.com/matomo.php"
      domains:
        "www3.example.com":
          trackingEnabled: true
          idSite: 21
          trackedMethods:
            - "GET"
          excludedPaths:
            - "/admin/*"
            - "\\.\\w{1,5}(\\?.+)?$"
          includedPaths:
            - "\\.(php|aspx)(\\?.*)?$"
          pathOverrides:
            "/subdir":
              trackingEnabled: true
              idSite: 24
              excludedPaths:
                - "/test"
                - "/test2"
            "/subdir2":
              trackingEnabled: false
              idSite: 24
        "test.de":
          trackingEnabled: false
          idSite: 456
```

**Explanation:**

- `matomo-tracking`: This is the name of the plugin instance in Traefik.
- `plugin`: Denotes that the following configuration is for a plugin.
- `matomoTracking`: The name of your custom plugin. It references the code logic that you provided in your Go plugin implementation.
- `matomoURL`: Specifies the Matomo server's URL where the tracking data should be sent.
- `domains`: Contains specific tracking configurations for multiple domains:
    - `"www3.example.com"`:
        - `trackingEnabled: true`: Enables tracking for `www3.example.com.`
        - `idSite: 21`: Uses `21` as the Matomo site ID.
        - `trackedMethods: ["GET"]`: Tracks only GET requests. HEAD, OPTIONS, and other methods are still forwarded but do not generate Matomo pageviews. Omit this setting to track all methods.
        - `excludedPaths`: Specifies paths that should not be tracked. For example:
            - `/admin/*`: Excludes all paths under `/admin`.
            - `\\.\\w{1,5}(\\?.+)?$`: Excludes files with extensions between 1 and 5 characters, and optionally followed by query parameters.
        - `includedPaths`: Specifies paths that should be tracked, even if they are excluded. 
        For example:
            - `\\.(php|aspx)(\\?.*)?$`: Includes files with extensions `.php` and `.aspx`, and optionally followed by query parameters
        - `pathOverrides`: The pathOverrides block allows you to define more specific tracking settings for particular path prefixes under the domain. These overrides take precedence over the domain-level settings, but only for requests that match the specified paths.
        Matching is based on prefix matching with path boundary awareness, meaning:
          - `/subdir` matches `/subdir` and `/subdir/test`, but not `/subdir2`.
        
          For example:
          - `"/subdir"`:
            - `trackingEnabled: true`: Enables tracking for paths under `/subdir`, even if other paths are excluded.
            - `idSite: 24`: Overrides the site ID used for this specific path.
            - `excludedPaths`: Within `/subdir`, `/subdir/test` and `/subdir/test2` are excluded from tracking.
          - `"/subdir2"`:
            - `trackingEnabled: false`: Disables tracking entirely for any request under `/subdir2`.
            - `idSite: 24`: Still defines an ID, but it is unused since tracking is disabled here.

    - `"test.de"`:
        - `trackingEnabled: false`: Disables tracking for `test.de`.
        - `idSite: 456`: Uses `456` as the Matomo site ID.


## Project Structure

Traefik requires `Config`, `CreateConfig` and `New` in the root package of a plugin. The root package only holds this entry point; the implementation lives in packages under `internal/`:

| Path | Responsibility |
| --- | --- |
| `matomotracking.go` | Plugin entry point for Traefik (`Config`, `CreateConfig`, `New`). |
| `internal/config` | Configuration types and merging of path overrides into domain settings. |
| `internal/rules` | Compiles the configuration once at startup (regular expressions, path overrides sorted by specificity), resolves the rules for a host and path, and decides whether a request is tracked. |
| `internal/tracker` | Captures the page view of a request and sends it to the Matomo tracking API. |
| `internal/middleware` | The `http.Handler`: forwards the request, records the response status and triggers tracking. |
| `internal/logger` | Log output with the middleware name; debug lines only with `debug: true`. |

### Request Processing

1. Determine the host (lower-case, without port) and look up its rules: the entry in `domains`, otherwise the top-level rules.
2. If tracking is disabled for the host, forward the request untouched.
3. Pick the most specific path override (longest prefix match with boundary awareness).
4. Check `trackingEnabled`, `trackedMethods`, `excludedPaths` and `includedPaths`. If the request does not qualify, forward it untouched.
5. Capture the page view (URL with lower-cased path, `User-Agent`, `X-Forwarded-For` chain), so later handlers in the chain cannot change what is tracked.
6. Forward the request. Only if response conditions are configured, the response writer is wrapped to record the final status code (see [docs/response-conditions.md](docs/response-conditions.md) for the effect on streamed responses); otherwise Traefik gets the original writer, so streaming and WebSockets behave as without the plugin.
7. If the response conditions match (or none are set), send the tracking request to Matomo asynchronously. It always uses GET, regardless of the original client method.

### Configuration Errors

The configuration is validated when Traefik loads the middleware: `matomoURL` must be an absolute URL, and every pattern in `excludedPaths`/`includedPaths` must be a valid regular expression. An invalid configuration is logged as an error and the middleware then forwards all requests **without tracking**. The sites stay reachable instead of Traefik disabling every router that uses the middleware.

## Setup instructions

Step 1: **Load/import the plugin into traefik**

1. Edit your Traefik static configuration file (e.g., traefik.yml or traefik.toml), and add the plugin's Github repository:

    Example: `traefik.yml`:
    ```yaml
    experimental:
      plugins:
        matomoTracking:
          moduleName: "github.com/DIE-Bonn/MatomoTracking"
          version: "v1.1.0"
    ```
**Ensure to use the current version tag.**

Step 2: **Configure Dynamic Configuration**

1. Create a new or use an already existing dynamic configuration file (e.g., dynamic.yml) that defines how the plugin should behave:

    Example `dynamic.yml`:
    ```yaml
    http:
      middlewares:
        matomo-tracking:
          plugin:
            matomoTracking:
              matomoURL: "https://matomo-staging.die-bonn.de/matomo.php"
              domains:
                "www3.die-bonn.de":
                  trackingEnabled: true
                  idSite: 21
                  excludedPaths:
                    - "/admin/*"
                    - "\\.\\w{1,5}(\\?.+)?$"
                  includedPaths:
                    - "\\.(php|aspx)(\\?.*)?$"
                  pathOverrides:
                    "/subdir":
                      trackingEnabled: true
                      idSite: 24
                      excludedPaths:
                        - "/test"
                        - "/test2"
                    "/subdir2":
                      trackingEnabled: false
                      idSite: 24
                "kansas-suche.de":
                  trackingEnabled: false
                  idSite: 456
    ```

    - This configuration defines the global rules for the `matomo-tracking` middleware, consisting of domain names with their individual tracking configuration.

    Alternatively, define the middleware per container with Docker labels and attach it to the container's router, see [docs/docker-labels.md](docs/docker-labels.md):

    ```yaml
    labels:
      - "traefik.http.routers.shop.middlewares=shop-matomo"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.matomoURL=https://matomo.example.com/matomo.php"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.trackingEnabled=true"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.idSite=3"
    ```
    In that case, skip step 3 for this container.

Step 3: **Associate the middleware plugin to the entrypoint**

1. Edit your Traefik static configuration file `traefik.yml`:

    Example `traefik.yml`:

    ```yaml
    entryPoints:
      webinsecure:
        address: ":80"
        http:
          middlewares:
            - matomo-tracking@file
    ```

    - This configuration ensures that the `matomo-tracking` plugin can analyze all incoming requests to decide which requests will be sent to the matomo server for tracking purposes.

Step 4: **Restart Traefik**

1. Start or restart traefik to load the plugin and apply the new configuration

    ```bash
    docker compose down && docker compose up -d
    ```

## Tests

Unit tests live alongside the source files in each package (e.g. `internal/rules/rules_test.go`). This follows Go's package structure and lets them test private functions without exposing them as public API.

Integration tests live in `tests/integration/` and use the plugin's public API. They require a running Matomo instance with site ID `1`. The default URL is `http://localhost:8082`; set `MATOMO_URL` to use a different instance.

```bash
# Unit tests only (no Matomo required)
go test -v . ./internal/...

# Integration tests only (requires Matomo)
go test -v ./tests/integration

# All tests (requires Matomo)
go test -v ./...
```

The same commands are available as VS Code tasks: `go test (unit)`, `go test (integration)`, and `go test`.

## Further Documentation

- Response-based tracking conditions: [docs/response-conditions.md](docs/response-conditions.md)
- Configuration via Docker labels: [docs/docker-labels.md](docs/docker-labels.md)
