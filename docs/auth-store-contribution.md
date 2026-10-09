# Shared authentication store follow-up for PR #117

Based on `03289559d547911d92ad58837db98faeb0c5fd8e` (develop history).
This patch preserves the original module and pinned whatsmeow-lib submodule
`0923702fb3fac8525241f15331b92116485d69eb`; no dependency update is required.

Credit: PR #117 by guilhermeCassettari and its recovery contribution by Ay0rus;
sharing proposals #194/#102 and existing-authDB proposals #174/#178/#206 informed
this independent implementation. Their branches are not merged here.

The store belongs to each service, while PostgreSQL belongs to the entrypoint.
PostgreSQL must borrow authDB with its configured limits; a missing handle is an
explicit error. SQLite sessions remain in dbdata/main.db, separate from users.db,
with one connection, foreign keys, WAL and busy_timeout enabled.

Only a successful Upgrade is published. Concurrent callers share one in-flight
initialization; no mutex is held during database I/O. Initialization is bounded
to 30 seconds and belongs to the store context. Cancelling one waiter does not
cancel other callers; authStore.close(ctx) cancels initialization and only closes
owned SQLite resources. The entrypoint uses a 10-second PingContext and closes
a failed PostgreSQL handle.

The original retry regression is retained and now closes its fixtures before
TempDir cleanup. Further regressions cover 32 concurrent callers, independent
cancellation, shutdown during initialization, deadlines, real SQLite sessions,
and 20 cycles of 32 calls against the same PostgreSQL pool.

Run from this PR checkout (Go 1.25, CGO and a C compiler):

```sh
git submodule update --init whatsmeow-lib
go build ./...
go vet ./pkg/whatsmeow/service
go test -timeout 5m ./pkg/whatsmeow/service
go test -race -timeout 5m ./pkg/whatsmeow/service
```

Set AUTH_STORE_TEST_POSTGRES_DSN to an isolated PostgreSQL 16 test database URL
before the test commands to enable the pool regression. The test role needs
CREATEDB: a temporary database avoids schema-agnostic table detection in the
pinned migration helper. The temporary database is dropped after handles close.
Without the variable, this one integration test explicitly skips.

Scope limitation: this patch does not solve the existing client-map races,
boolean stop-channel ownership, worker shutdown or automatic reconnect policy.
Do not call authStore.close while legacy session workers are using it. The full
fork has a separate lifecycle patch that quiesces consumers before closing stores;
that change is intentionally outside PR #117. No WhatsApp pairing/reconnection,
passkey or real messages were used to validate this storage patch.
