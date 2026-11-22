-- Distinct visitors of a refused path: the gateway sends a keyed hash per
-- client address (never the address) and the panel keeps up to 16 of them,
-- so that one visitor coming back is counted once. The old "clients" column
-- summed every gateway batch and is kept only for reports of older gateways.
ALTER TABLE unmapped_hits ADD COLUMN visitors text[] NOT NULL DEFAULT '{}';
-- A summed count is not a number of visitors: until a gateway sends keys for
-- a path, it counts as one, so that old rows never pass the agent's
-- two-visitor threshold on their own.
UPDATE unmapped_hits SET clients = LEAST(clients, 1);
