CREATE TABLE IF NOT EXISTS listener_android_fcm_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    device_token TEXT NOT NULL UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS listener_android_fcm_devices_listener_id_idx
    ON listener_android_fcm_devices(listener_id);
