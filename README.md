# `caddy-tlsfirestore`

## What is this?

A [Caddy](https://caddyserver.com/) module that uses Google
[firestore](https://cloud.google.com/firestore) for storing TLS certificates
instead of the local file system.

## Why is this?

I needed it for [falsifiable](https://falsifiable.com). I wanted users to have ownership
over their content. That demands that they own the addressing. If you don't want to 
allow insecure connections, this requires TLS. Caddy does most of that ACME heavy lifting.
The only thing missing (for me) was more secure secret storage on Google Cloud.

I toyed with using Google Secrets Manager for storing all the certificates. But distributed
locking is easier with transactions than it is with bad secrets manager kludges. 

Following the lead of `caddy-tlsconsul`, All certificates are encrypted using AES in GCM.
The stored value is prefixed with the randomly sampled nonce. Technically, this is 
bad nonce. But, a 12 byte nonce for such a small set of objects is very unlikely to 
have collision problems. 

Unlike `caddy-tls` you cannot opt out of encryption. Also, since I don't like storing
secrets in environmental variables or configuration files, you can choose to use
Google Secrets Manager for the encryption key.

### Credits

Inspired by [`j0hnsmith`'s](https://github.com/pteich) [`caddy-tlsclouddatastore`](https://github.com/j0hnsmith/caddy-tlsclouddatastore) 
which was inspired by [`pteich`'s](https://github.com/pteich) [`caddy-tlsconsul`](https://github.com/pteich/caddy-tlsconsul).
