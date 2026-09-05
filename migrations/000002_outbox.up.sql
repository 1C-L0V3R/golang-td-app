CREATE TABLE todoapp.outbox (
    seq            BIGSERIAL                PRIMARY KEY,
    id             UUID          NOT NULL   UNIQUE,
    aggregate_type TEXT          NOT NULL   CHECK (aggregate_type IN ('user', 'task')),
    aggregate_id   TEXT          NOT NULL,
    event_type     TEXT          NOT NULL   CHECK (event_type IN ('created', 'updated', 'deleted', 'snapshot')),
    version        BIGINT        NOT NULL,
    payload        JSONB         NOT NULL,
    created_at     TIMESTAMPTZ   NOT NULL   DEFAULT now(),
    published_at   TIMESTAMPTZ,
    attempts       INTEGER       NOT NULL   DEFAULT 0,
    last_error     TEXT
);

CREATE INDEX outbox_unpublished_idx ON todoapp.outbox (seq) WHERE published_at IS NULL;
