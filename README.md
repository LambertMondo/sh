# camerind.com reverse proxy on Upsun/Platform.sh

This app exposes an Upsun/Platform.sh route and forwards every HTTP request to
`https://camerind.com`. Client TLS is terminated by the platform router; the Go
app creates a separate, certificate-verified TLS connection to camerind.com.

The proxy preserves paths, query strings, request bodies, streaming responses,
and protocol upgrades. It sends `Host: camerind.com` upstream and rewrites
upstream redirects and domain cookies back to the public platform domain.

## Configuration

`TARGET_URL` controls the fixed upstream and defaults through the deployment
configuration to:

```text
https://camerind.com
```

Both platform configuration layouts are included:

- Platform.sh/Upsun Fixed: `.platform.app.yaml` and `.platform/routes.yaml`
- Upsun: `.upsun/app.yaml`, `.upsun/routes.yaml`, and `.upsun/services.yaml`

Do not add the same routes to a second `.upsun/config.yaml`; Upsun combines
recognized configuration files and rejects duplicate route names.

## Deploy

Configure the project Git remote, then push the current commit/branch:

```bash
git remote add platform <your-platformsh-git-remote>
git push platform HEAD
```

After deployment, opening the generated `*.platformsh.site` URL should return
the camerind.com application while the browser remains on the platform URL.

This is an HTTP reverse proxy, not an IP mirror: camerind.com is resolved by
DNS and contacted from the platform container for each new upstream connection.
