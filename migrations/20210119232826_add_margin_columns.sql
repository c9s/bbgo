-- +up
ALTER TABLE `trades`
    ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE
    ;

ALTER TABLE `orders`
    ADD COLUMN `is_margin` BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN `is_isolated` BOOLEAN NOT NULL DEFAULT FALSE
    ;

-- +down
ALTER TABLE `trades`
    DROP COLUMN `is_margin`,
    DROP COLUMN `is_isolated`;

ALTER TABLE `orders`
    DROP COLUMN `is_margin`,
    DROP COLUMN `is_isolated`;
