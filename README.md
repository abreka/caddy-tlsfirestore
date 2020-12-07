# `caddy-tlsfirestore`

## What is this?

A [Caddy](https://caddyserver.com/) module that uses Google
[firestore](https://cloud.google.com/firestore) for storing TLS certificates
instead of the local file system.

## Usage

The sample `Dockerfile` in the repo shows basic usage for a pre-compiled binary. If
you are using it with Google Secrets Manager to store the AES Key, 
your `Caddyfile` would look something like,

```Caddyfile
{
    storage firestore {
           project_id             "GCP_PROJECT_NAME"
           collection             "FIRESTORE_COLLECTION_NAME"
           aes_key_secret_id      "AES_KEY_SECRET_ID"
    }
}

# ...
```

otherwise, use,

```Caddyfile
{
    storage firestore {
           project_id             "GCP_PROJECT_NAME"
           collection             "FIRESTORE_COLLECTION_NAME"
    }
}

# ...
```

with the environmental variable `CADDY_CLUSTERING_AESKEY_BASE64` set to the base64-encoded
AES private key. (If you are using secrets, store it as a blob without base64 encoding).

Then for each domain, add an entry like the following,

```Caddyfile
CUSTOMER_DOMAIN:443 {
    reverse_proxy https://INTERNALDOMAIN {
		header_up Proxied yes
        header_up Host {http.reverse_proxy.upstream.host}
    }
}
```

Caddy will then automatically provision TLS using [Let's Encrypt](https://letsencrypt.org/)
(renewals too).

You still need a scheme for registering the customer domains in the Caddy file. I may 
add mine to this repo once I'm more confident with it. However, on straight-forward solution
is running `caddy` with the `-watch` flag active, and rewriting the file for new registrations.

## Why is this?

I needed it for [falsifiable](https://falsifiable.com). I 
[want users to have ownership over their content](https://dev.falsifiable.com/progress/an-origin-story).
Near term, that demands that they own the location-based addressing. If you want secure connections,
this requires TLS. 
[Caddy 2 does most of that ACME heavy lifting](https://caddyserver.com/docs/automatic-https).
The only thing missing (for me) was secure and distributed secret storage using 
[Google Cloud](https://cloud.google.com/).

I toyed with using [Google Secrets Manager](https://cloud.google.com/secret-manager) for 
storing all the certificates. But the interface of Caddy 2 makes that difficult. Moreover,
for deployments on a cluster, distributed locking is more of a kludge with Secrets Manager
whereas it's easy(er) with firestore.

Following the lead of `caddy-tlsconsul`, All certificates are encrypted using AES-GCM.
The stored value is prefixed with the randomly sampled nonce. (Technically, this is 
bad nonce. But, with 12 byte of entry for each on such a small set of objects it 
is very unlikely to have a collision.)

Unlike `caddy-tlsconsul` *you cannot opt out of encryption*. Also, since I don't like storing
secrets in environmental variables or configuration files, you can choose to use
Google Secrets Manager for the encryption key.

### Credits

Inspired by [j0hnsmith's](https://github.com/pteich) [`caddy-tlsclouddatastore`](https://github.com/j0hnsmith/caddy-tlsclouddatastore) 
which was inspired by [pteich's](https://github.com/pteich)
[`caddy-tlsconsul`](https://github.com/pteich/caddy-tlsconsul).
