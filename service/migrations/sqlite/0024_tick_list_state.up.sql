-- Tick list completion state (shared across clients, keyed by table config name and item id)
CREATE TABLE IF NOT EXISTS tick_list_state (
    tc_name VARCHAR(255) NOT NULL,
    item_id VARCHAR(255) NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT 0,
    PRIMARY KEY (tc_name, item_id)
);
CREATE INDEX IF NOT EXISTS idx_tick_list_state_tc_name ON tick_list_state(tc_name);
