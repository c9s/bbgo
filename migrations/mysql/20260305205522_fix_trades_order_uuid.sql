-- +up
-- +begin
ALTER TABLE `trades` MODIFY COLUMN `order_uuid` VARBINARY(36) NOT NULL DEFAULT '';
-- +end

-- +down
-- +begin
ALTER TABLE `trades` MODIFY COLUMN `order_uuid` VARBINARY(16) NOT NULL DEFAULT '';
-- +end
