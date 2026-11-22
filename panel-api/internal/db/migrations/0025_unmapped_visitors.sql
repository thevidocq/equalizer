-- Distinct visitors of a refused path: the gateway sends a keyed hash per
-- client address (never the address) and the panel keeps up to 16 of them,
-- so that one visitor coming back is counted once. The old "clients" column
-- summed every gateway batch and is kept only for reports of older gateways.
ALTER TABLE unmapped_hits ADD COLUMN visitors text[] NOT NULL DEFAULT '{}';
