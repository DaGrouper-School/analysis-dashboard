# Operations

Running the service once it is deployed. Configuration is in [Development](development.md#configuration).

## Signing keys

User tokens are signed with Ed25519 keys from the key file `DGS_SIGNING_KEYS_FILE`. Outside development and test, start-up is refused without one. Keep it as a secret, readable only by the service.

```json
{"keys": [
  {"kid": "20261123-3f9a1c", "seed": "<32-byte seed, unpadded base64url>", "signs": true}
]}
```

- Exactly one key signs. Every key in the file is published in the key set at `/api/.well-known/jwks.json`, so SAM verifies a token signed by any of them.
- `dgs keys new` prints a new key, not yet signing, named by the day and a random suffix.
- Every instance reads the file at start-up, so a change takes effect as each one restarts.

### Rotating

SAM keeps a fetched key set for five minutes. A key is therefore published before it signs, and stays published after:

1. Add the key `dgs keys new` prints, with `signs` false, and restart every instance. The set now publishes it.
2. At least five minutes later, move `signs: true` to the new key and restart every instance. SAM's cached set already holds the new key, and tokens signed by the old one still verify.
3. A day later, remove the old key and restart.

### A lost or leaked key

Tokens last five minutes, so a leaked key is useful only while it is published. Add a new key and make it sign at once, removing the leaked one in the same change, then restart every instance. SAM refetches the set when a token names a key it does not hold, at most once every thirty seconds, so calls made as a person may be refused for up to half a minute.

## Held changes

Changes reach SAM through the outbox. A change SAM refuses, whose actor's account is gone, or that this version cannot read is held, and logged at warning level as `outbox change held`. Later changes to the same entity are still sent, and one that reaches SAM supersedes the held one.

```sh
dgs outbox held                # what is held: the entity, kind, version, who it is sent as, attempts and the error
dgs outbox release ENTITY      # send the entity's latest held change again on the worker's next pass
```

Release a change once what refused it is fixed, for example once the person has the rights SAM asked for. A released change that SAM refuses again is held again.

## After a database restore

A database restored from a backup has forgotten the accounts made after the backup, but SAM still knows their participant IDs. At start-up the restore guard moves the participant ID sequence past every ID SAM knows, and logs `participant ID guard moved the sequence`. New accounts then get IDs SAM has never seen.

An account made after the restore but before the guard ran, such as one that signed up while SAM could not be asked, may have an ID SAM knows as someone else's. Its registration is held, never adopted, with the error saying so: `dgs outbox held` lists it as `user:<user id>`. Give it a new ID, and its registration is queued again:

```sh
dgs accounts renumber USER
```

Renumbering refuses an account SAM already has, whose ID is settled.

## The tenant file

`DGS_SAM_TENANT_FILE` is what `sam tenant create -json` printed when this service was onboarded, and holds its service key, shown that once. Keep it as a secret. Onboarding again for an onboarded tenant prints no key: a lost service key is replaced with an emergency key issued by SAM's operator ([SAM's tenants](https://github.com/DaGrouper-School/sam/blob/main/docs/tenants.md)).
