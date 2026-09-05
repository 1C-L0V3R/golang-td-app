INSERT INTO todoapp.outbox (id, aggregate_type, aggregate_id, event_type, version, payload)
SELECT
    gen_random_uuid(),
    'user',
    u.id::text,
    'snapshot',
    u.version,
    jsonb_build_object(
        'id',           u.id,
        'version',      u.version,
        'full_name',    u.full_name,
        'phone_number', u.phone_number
    )
FROM todoapp.users u;

INSERT INTO todoapp.outbox (id, aggregate_type, aggregate_id, event_type, version, payload)
SELECT
    gen_random_uuid(),
    'task',
    t.id::text,
    'snapshot',
    t.version,
    jsonb_build_object(
        'id',             t.id,
        'version',        t.version,
        'title',          t.title,
        'description',    t.description,
        'completed',      t.completed,
        'created_at',     t.created_at,
        'completed_at',   t.completed_at,
        'author_user_id', t.author_user_id
    )
FROM todoapp.tasks t;
