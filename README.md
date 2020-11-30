# `caddy-tls-datastore`

## What is this?

A [Caddy](https://caddyserver.com/) module that uses Google
[Secrets Manager](https://cloud.google.com/secret-manager) for storing TLS certificates
instead of the local file system.

## Why is this?

I needed it for [falsifiable](https://falsifiable.com). I wanted users to have ownership
over their content. That demands that they own the addressing. If you don't want to 
allow insecure connections, this requires TLS. Caddy does most of that ACME heavy lifting.
The only thing missing (for me) was more secure secret storage on Google Cloud.

### Credits

Inspired by [`j0hnsmith`'s](https://github.com/pteich) [`caddy-tlsclouddatastore`](https://github.com/j0hnsmith/caddy-tlsclouddatastore) 
which was inspired by [`pteich`'s](https://github.com/pteich) [`caddy-tlsconsul`](https://github.com/pteich/caddy-tlsconsul).
