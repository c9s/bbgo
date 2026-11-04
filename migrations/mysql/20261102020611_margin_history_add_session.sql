-- +up
-- +begin
ALTER TABLE `margin_loans`
    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',
    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,
    DROP INDEX `transaction_id`,
    ADD UNIQUE KEY `transaction_id_session` (`transaction_id`, `session`);
-- +end

-- +begin
ALTER TABLE `margin_repays`
    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',
    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,
    DROP INDEX `transaction_id`,
    ADD UNIQUE KEY `transaction_id_session` (`transaction_id`, `session`);
-- +end

-- +begin
ALTER TABLE `margin_liquidations`
    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',
    DROP INDEX `order_id`,
    ADD UNIQUE KEY `order_id_exchange_session` (`order_id`, `exchange`, `session`);
-- +end

-- +begin
ALTER TABLE `margin_interests`
    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN `price_in_usdt` DECIMAL(32, 16) UNSIGNED NOT NULL DEFAULT 0,
    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,
    MODIFY `interest` DECIMAL(32, 16) UNSIGNED NOT NULL;
-- +end

-- +begin
CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);
-- +end

-- +down

-- Rows of several sessions can share a key once the old keys come back. Each
-- DELETE keeps the row with the smallest session name per old key (legacy ''
-- first) and deletes only the other copies, so history synced after the
-- upgrade is kept.

-- +begin
DROP INDEX `idx_margin_interests_session_time` ON `margin_interests`;
-- +end

-- margin_interests has no unique key, but the session copies would double count.
-- A copy of the same charge matches in every column. Two accounts are charged at
-- the same hour too, so rows count as copies only when the principal, interest
-- and rate match as well, and the rows of other accounts are kept.
-- +begin
DELETE a FROM `margin_interests` a
    JOIN `margin_interests` b
        ON a.`exchange` = b.`exchange`
        AND a.`asset` = b.`asset`
        AND a.`isolated_symbol` = b.`isolated_symbol`
        AND a.`time` = b.`time`
        AND a.`principle` = b.`principle`
        AND a.`interest` = b.`interest`
        AND a.`interest_rate` = b.`interest_rate`
        AND b.`session` < a.`session`;
-- +end

-- +begin
ALTER TABLE `margin_interests` DROP COLUMN `price_in_usdt`, DROP COLUMN `session`;
-- +end

-- +begin
DELETE a FROM `margin_liquidations` a
    JOIN `margin_liquidations` b
        ON a.`order_id` = b.`order_id`
        AND a.`exchange` = b.`exchange`
        AND b.`session` < a.`session`;
-- +end

-- +begin
ALTER TABLE `margin_liquidations`
    DROP INDEX `order_id_exchange_session`,
    ADD UNIQUE KEY `order_id` (`order_id`, `exchange`),
    DROP COLUMN `session`;
-- +end

-- +begin
DELETE a FROM `margin_repays` a
    JOIN `margin_repays` b
        ON a.`transaction_id` = b.`transaction_id`
        AND b.`session` < a.`session`;
-- +end

-- +begin
ALTER TABLE `margin_repays`
    DROP INDEX `transaction_id_session`,
    ADD UNIQUE KEY `transaction_id` (`transaction_id`),
    DROP COLUMN `session`;
-- +end

-- +begin
DELETE a FROM `margin_loans` a
    JOIN `margin_loans` b
        ON a.`transaction_id` = b.`transaction_id`
        AND b.`session` < a.`session`;
-- +end

-- +begin
ALTER TABLE `margin_loans`
    DROP INDEX `transaction_id_session`,
    ADD UNIQUE KEY `transaction_id` (`transaction_id`),
    DROP COLUMN `session`;
-- +end
