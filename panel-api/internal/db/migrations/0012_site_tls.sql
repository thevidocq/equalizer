-- How visitors reach a site (https with its own certificate, or http as the
-- legacy site was) and the certificate the customer uploaded: the legacy
-- site's own, never one issued by the platform. The private key is sealed
-- with the panel's secret key and never leaves the panel except, in clear,
-- to the gateways on the control network.
ALTER TABLE apps ADD COLUMN public_scheme text NOT NULL DEFAULT 'https'
    CHECK (public_scheme IN ('https', 'http'));

CREATE TABLE app_certificates (
    app_id      text PRIMARY KEY REFERENCES apps (id) ON DELETE CASCADE,
    chain_pem   text NOT NULL,
    key_sealed  bytea NOT NULL,
    subject     text NOT NULL,
    issuer      text NOT NULL,
    domains     text[] NOT NULL,
    not_before  timestamptz NOT NULL,
    not_after   timestamptz NOT NULL,
    fingerprint text NOT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    uploaded_by bigint REFERENCES users (id) ON DELETE SET NULL
);
