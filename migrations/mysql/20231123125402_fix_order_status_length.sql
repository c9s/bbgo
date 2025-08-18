-- +up
-- +begin
ALTER TABLE `orders`
    CHANGE `status` `status` varchar(20) NOT NULL;
-- +end

-- +down

-- +begin
SELECT 1;
-- +end
