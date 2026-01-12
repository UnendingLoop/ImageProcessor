CREATE TABLE IF NOT EXISTS images (
    image_uid TEXT PRIMARY KEY,
    source_key TEXT NOT NULL,
    wm_key TEXT,
    result_key TEXT,
    operation TEXT NOT NULL,
    CHECK (
        operation IN (
            'resize',
            'watermark',
            'thumbnail'
        )
    ),
    x_axis INT,
    y_axis INT,
    status TEXT NOT NULL,
    CHECK (
        status IN (
            'created',
            'in_progress',
            'failed',
            'done'
        )
    ),
    err_msg JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);