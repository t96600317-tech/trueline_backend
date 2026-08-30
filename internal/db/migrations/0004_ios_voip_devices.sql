-- iOS listener devices eligible to receive APNs VoIP pushes for incoming calls.
CREATE TABLE IF NOT EXISTS listener_ios_voip_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    device_token TEXT NOT NULL UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_listener_ios_voip_devices_listener_id
    ON listener_ios_voip_devices(listener_id);
