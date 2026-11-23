# Migrations

SQL files applied in name order, each in its own transaction, by `dgs serve` before it serves and by `dgs migrate`.

- A new table or column is a new migration, named `NNNN_what_it_adds.sql` with the next number. An applied migration is never edited.
- It starts with a comment that says why.
- It must not break the release before it, which keeps running during an upgrade: a column is added before code reads it, and dropped only once no released code uses it.
- Each table it creates belongs to one package, listed in `docs/development.md` under "Table ownership"; a test fails otherwise.
