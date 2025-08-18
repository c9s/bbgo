# Environment Variables

## MAX Exchange

```shell
# MAX_QUERY_CLOSED_ORDERS_NUM_OF_PAGES=[number of pages]
# The MAX Exchange API does not support time-range based query for the closed orders
# We can only sync the closed orders by page number, here is the maximum pages you want to sync
MAX_QUERY_CLOSED_ORDERS_NUM_OF_PAGES=10


# MAX_QUERY_CLOSED_ORDERS_LIMIT=[limit per query]
# using default limit 1000 might cause the server response timeout, you can decrease this limit to prevent this kind of error.
# default = 1000
MAX_QUERY_CLOSED_ORDERS_LIMIT=500


# MAX_QUERY_CLOSED_ORDERS_ALL=[1 or 0]
# The MAX Exchange API does not support time-range based query for the closed orders
# If you want to sync all the orders, you must start from the first page
# To enable this mode, set this variable to 1
MAX_QUERY_CLOSED_ORDERS_ALL=1
```

