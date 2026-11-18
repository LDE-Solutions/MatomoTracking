# Configuration via Docker labels

Besides the dynamic configuration file, the middleware can be defined directly on a container
with Docker labels. Each container then carries its own tracking settings, and only the routers
that reference the middleware are tracked.

Summary
- Labels define a regular Traefik plugin middleware: `traefik.http.middlewares.<name>.plugin.matomoTracking.<option>=<value>`.
- Use the top-level options (`trackingEnabled`, `idSite`, ...) instead of `domains`. They apply to every host routed through the middleware.
- `domains` is not usable in labels, because host names contain dots and Traefik splits label keys at dots.
- The plugin name (`matomoTracking`) must match the name under `experimental.plugins` (or `localPlugins`) in the static configuration.

Example (`compose.yml`)
```yaml
services:
  shop:
    image: traefik/whoami:v1.11
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.shop.rule=Host(`shop.example.com`)"
      - "traefik.http.routers.shop.middlewares=shop-matomo"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.matomoURL=https://matomo.example.com/matomo.php"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.trackingEnabled=true"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.idSite=3"
      # Lists are comma-separated
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.trackedMethods=GET,POST"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.excludedPaths=\\.(css|js|png)$$"
      # Path overrides: the path prefix is part of the label key
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.pathOverrides./checkout.idSite=4"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.pathOverrides./admin.trackingEnabled=false"
      # Response conditions
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.responseConditions.trackOnStatusCodes=200"
      - "traefik.http.middlewares.shop-matomo.plugin.matomoTracking.responseConditions.trackWhenHeaders.Content-Type=text/html; charset=utf-8"
```

Available options
| Label suffix | Example value | Notes |
| --- | --- | --- |
| `matomoURL` | `https://matomo.example.com/matomo.php` | Required. |
| `trackingEnabled` | `true` | Required to track; defaults to `false`. |
| `idSite` | `3` | Matomo site ID. |
| `trackedMethods` | `GET,POST` | Comma-separated. Omit to track all methods. |
| `excludedPaths` / `includedPaths` | `\.css$$,\.js$$` | Comma-separated regular expressions, so a pattern cannot contain a comma. |
| `pathOverrides.<prefix>.<option>` | `pathOverrides./shop.idSite=4` | Same options as above, except `matomoURL` and `pathOverrides`. |
| `responseConditions.trackOnStatusCodes` | `200,201` | Comma-separated. |
| `responseConditions.trackWhenHeaders.<Header>` | `...trackWhenHeaders.Content-Type=text/html` | |
| `debug` | `true` | Logs every tracking decision. |

Notes and limitations
- In a Compose file, `$` must be written as `$$`.
- Path override prefixes cannot contain dots (e.g. `/v1.2`), because the prefix is part of the label key. Use the file provider for such paths.
- Path prefixes are matched case-sensitively, as in the file configuration.
- Combining a label middleware with a middleware on the entry point is fine as long as the entry point middleware does not track the same host: it only tracks hosts listed in its `domains` (and, if set, everything via its own top-level options). Otherwise a request is tracked twice.
- An invalid configuration (e.g. a broken regular expression or a missing `matomoURL`) is logged when Traefik loads the middleware. The middleware then forwards requests without tracking, so the site stays reachable.
