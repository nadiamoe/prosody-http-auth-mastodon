# prosody-http-auth-mastodon

This tiny server implements the interface defined by Prosody's [mod_auth_custom_http](https://modules.prosody.im/mod_auth_custom_http.html), and checks authentication data against a Postgres database which is assumed to be the one that [Mastodon](https://github.com/mastodon/mastodon/) uses. Effectively, this allows XMPP users to log in with their mastodon username/password.

Integration tests are included, and the schema is assumed to be that of Mastodon v4.3.3, or compatible with it.

## Usage

Run the container, which can be configured with the following env vars:
- `DB_URL` (required): URL to connect to the postgres database. Something like: `postgresql://user:password@host/mastodon_production?sslmode=disable`.
  - `DB_URL` can contain references to other environment variables, which will be expanded: `postgresql://${PG_USER}:${PG_PASSWORD}@host/mastodon_production?sslmode=disable`.
  - You must include `?sslmode=disable` if your postgres installation is not using TLS.
