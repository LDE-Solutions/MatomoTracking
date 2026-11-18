# Response-based tracking conditions

This feature defers the tracking decision until after the response has been generated. You can restrict tracking to specific HTTP status codes and/or response headers.

Summary
- Tracking is evaluated post-response.
- Conditions can be configured per domain and overridden per path.
- Backward compatible: if no conditions are set, behavior stays unchanged.

Configuration schema
- Config.responseConditions (top-level rules, e.g. for Docker labels)
- DomainConfig.responseConditions
- PathConfig.responseConditions
- ResponseConditions:
  - trackOnStatusCodes: list of allowed final status codes (empty = allow any)
  - trackWhenHeaders: required response headers (exact key/value matches; header names are case-insensitive)

Evaluation order
1) Domain enabled (trackingEnabled).
2) Path include/exclude rules.
3) Response conditions after the handler writes the response.
   - All headers in trackWhenHeaders must be present with exactly matching values.
   - If trackOnStatusCodes is set, status must be one of the listed values.

Traefik dynamic config (YAML)
```yaml
http:
  middlewares:
    matomo-tracking:
      plugin:
        matomoTracking:
          matomoURL: "http://matomo-local/matomo.php"
          domains:
            "demo.localhost":
              trackingEnabled: true
              idSite: 1
              # Track only successful HTML pages
              responseConditions:
                trackOnStatusCodes: [200]
                trackWhenHeaders:
                  Content-Type: "text/html; charset=UTF-8"
```

Notes and limitations
- Conditions are ANDed (status AND all headers must match).
- Multi-value headers pass if any value equals the configured one.
- Header names are case-insensitive; values match exactly (no regex).
- Tracking is sent after response completion; long-running responses delay the send.
- WebSocket upgrades are evaluated with status `101`.
- Streamed responses (e.g. server-sent events) are buffered on routes with response conditions: to read the status, the middleware wraps the response writer, and Traefik's plugin interpreter (Yaegi) hides the wrapper's `Flush` from Traefik. WebSockets are not affected. Without response conditions the writer is passed on unchanged and streaming works normally. Exclude streaming endpoints from tracking (`excludedPaths`, or a path override with `trackingEnabled: false` or an empty `responseConditions: {}`) if they must stream.

Testing
- Unit tests: `internal/rules/rules_test.go`, run with `go test -v ./internal/...`.
- Integration tests: `tests/integration/response_conditions_test.go`, run with `go test -v ./tests/integration` (requires local Matomo).
- All tests: `go test -v ./...` (requires local Matomo).
