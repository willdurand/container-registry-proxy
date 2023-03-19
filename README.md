# Container Registry Proxy

[![CI](https://github.com/willdurand/container-registry-proxy/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/willdurand/container-registry-proxy/actions/workflows/ci.yml)

![](https://williamdurand.fr/images/posts/2023/03/container-registry-proxy.webp)

This is a container registry proxy that is mainly used to make the GitHub
Container Registry fully compatible with the [Docker Registry HTTP API V2
specification][http-api].

Important: This small application is designed for a specific use case in mind so
it is very likely that it isn't going to solve _your_ problems.

## Environment variables

- `GITHUB_TOKEN`: required - a GitHub (personal access) token with `read:packages` permission
- `HOST`: optional - the proxy address (default: `127.0.0.1`)
- `PORT`: optional - the proxy port (default: `10000`)
- `UPSTREAM_URL`: optional - the URL of the upstream container registry (default: `https://ghcr.io`)

## Quick start

1. Go to https://github.com/settings/tokens and generate a classic token with
   the `read:packages` scope.
2. Run the proxy using Docker:

   ```
   $ docker run --rm -e GITHUB_TOKEN=<personal access token> willdurand/container-registry-proxy
   2023/03/18 13:53:27 starting container registry proxy on 127.0.0.1:10000
   ```

## Docker on Synology

1. Go to https://github.com/settings/tokens and generate a classic token with
   the `read:packages` scope.
2. In _DSM_, create a new container using the `willdurand/container-registry-proxy`
   image available on the Docker Hub. Make sure to define a `GITHUB_TOKEN`
   environment variable with the value generated in the previous step. Also, add
   this container to the "host" network.
3. Next, configure a new registry in _DSM > Docker > Registry > Settings_, e.g.
   `GitHub Registry (Proxied)` with a dummy URL for now (we'll change it
   manually later), e.g. `http://nas.local:10000`. You must configure a GitHub
   username and the password should be the GitHub token generated previously.
4. SSH into the Synology and open `/var/packages/Docker/etc/registry.json` using
   elevated privileges (i.e. `sudo vim`). Change `nas.local` to `127.0.0.1`,
   save and quit.

At this point, the registry proxy is fully configured. In _DSM > Docker >
Registry > Settings_, select the newly added registry and click "Use". You
should now see the list of images.

## License

See the bundled [LICENSE](./LICENSE) file for details.

[http-api]: https://docs.docker.com/registry/spec/api/
